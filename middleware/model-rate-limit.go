package middleware

import (
	"context"
	"fmt"
	"net/http"
	"one-api/common"
	"one-api/common/limiter"
	"one-api/constant"
	"one-api/setting"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	ModelRequestRateLimitCountMark        = "MRRL"
	ModelRequestRateLimitSuccessCountMark = "MRRLS"
)

// Redis限流处理器 - 使用新的滑动窗口算法
func redisRateLimitHandler(duration int64, totalMaxCount, successMaxCount int, model string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userId := strconv.Itoa(c.GetInt("id"))
		ctx := context.Background()
		rdb := common.RDB

		// 初始化限流器
		rl := limiter.New(ctx, rdb)

		// 1. 检查成功请求数限制
		successKey := fmt.Sprintf("rateLimit:%s:%s:%s", ModelRequestRateLimitSuccessCountMark, userId, model)
		allowed, err := rl.SlidingWindowCheck(ctx, successKey, successMaxCount, duration)
		if err != nil {
			fmt.Printf("检查成功请求数限制失败: %v\n", err)
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "rate_limit_check_failed")
			return
		}
		if !allowed {
			abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("您已达到请求数限制：%d分钟内最多请求%d次", setting.ModelRequestRateLimitDurationMinutes, successMaxCount))
			return
		}

		// 2. 检查总请求数限制（当totalMaxCount为0时会自动跳过）
		if totalMaxCount > 0 {
			totalKey := fmt.Sprintf("rateLimit:%s:%s:%s", ModelRequestRateLimitCountMark, userId, model)
			allowed, err := rl.SlidingWindowRecord(ctx, totalKey, totalMaxCount, duration)
			if err != nil {
				fmt.Printf("检查总请求数限制失败: %v\n", err)
				abortWithOpenAiMessage(c, http.StatusInternalServerError, "rate_limit_check_failed")
				return
			}
			if !allowed {
				abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("您已达到总请求数限制：%d分钟内最多请求%d次，包括失败次数，请检查您的请求是否正确", setting.ModelRequestRateLimitDurationMinutes, totalMaxCount))
				return
			}
		}

		// 3. 处理请求
		c.Next()

		// 4. 如果请求成功，记录成功请求
		if c.Writer.Status() < 400 {
			_, err := rl.SlidingWindowRecord(ctx, successKey, successMaxCount, duration)
			if err != nil {
				fmt.Printf("记录成功请求失败: %v\n", err)
			}
		}
	}
}

// 内存限流处理器 - 保持原有逻辑
func memoryRateLimitHandler(duration int64, totalMaxCount, successMaxCount int, model string) gin.HandlerFunc {
	inMemoryRateLimiter.Init(time.Duration(setting.ModelRequestRateLimitDurationMinutes) * time.Minute)

	return func(c *gin.Context) {
		userId := strconv.Itoa(c.GetInt("id"))
		totalKey := ModelRequestRateLimitCountMark + userId + ":" + model
		successKey := ModelRequestRateLimitSuccessCountMark + userId + ":" + model

		// 1. 检查总请求数限制（当totalMaxCount为0时跳过）
		if totalMaxCount > 0 && !inMemoryRateLimiter.Request(totalKey, totalMaxCount, duration) {
			abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("您已达到总请求数限制：%d分钟内最多请求%d次，包括失败次数，请检查您的请求是否正确", setting.ModelRequestRateLimitDurationMinutes, totalMaxCount))
			return
		}

		// 2. 检查成功请求数限制
		// 使用一个临时key来检查限制，这样可以避免实际记录
		checkKey := successKey + "_check"
		if !inMemoryRateLimiter.Request(checkKey, successMaxCount, duration) {
			abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("您已达到请求数限制：%d分钟内最多请求%d次", setting.ModelRequestRateLimitDurationMinutes, successMaxCount))
			return
		}

		// 3. 处理请求
		c.Next()

		// 4. 如果请求成功，记录到实际的成功请求计数中
		if c.Writer.Status() < 400 {
			inMemoryRateLimiter.Request(successKey, successMaxCount, duration)
		}
	}
}

// ModelRequestRateLimit 模型请求限流中间件
func ModelRequestRateLimit() func(c *gin.Context) {
	return func(c *gin.Context) {
		// 在每个请求时检查是否启用限流
		if !setting.ModelRequestRateLimitEnabled {
			c.Next()
			return
		}

		// 计算限流参数
		duration := int64(setting.ModelRequestRateLimitDurationMinutes * 60)
		totalMaxCount := setting.ModelRequestRateLimitCount
		successMaxCount := setting.ModelRequestRateLimitSuccessCount

		// 获取分组
		group := common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
		if group == "" {
			group = common.GetContextKeyString(c, constant.ContextKeyUserGroup)
		}

		// 获取模型名称
		model := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)
		
		// 临时调试信息 - 可以在确认修复后删除
		fmt.Printf("DEBUG: 用户分组=%s, 模型=%s\n", group, model)
		
		if model == "" {
			// 如果没有模型信息，使用默认限制
			groupTotalCount, groupSuccessCount, found := setting.GetGroupRateLimit(group)
			if found {
				totalMaxCount = groupTotalCount
				successMaxCount = groupSuccessCount
			}
		} else {
			// 获取分组对特定模型的限流配置
			groupTotalCount, groupSuccessCount, found := setting.GetGroupModelRateLimit(group, model)
			if found {
				// 找到了模型特定的限制，使用该限制
				totalMaxCount = groupTotalCount
				successMaxCount = groupSuccessCount
			} else {
				// 如果没有找到模型特定的限制，检查是否有分组的一般限制
				groupTotalCount, groupSuccessCount, found = setting.GetGroupRateLimit(group)
				if found {
					// 检查该分组是否配置了模型列表
					if setting.HasGroupModelList(group) {
						// 如果分组配置了模型列表，但当前模型不在列表中，则不进行限制
						// 直接跳过限流检查
						fmt.Printf("DEBUG: 模型 %s 不在分组 %s 的限制列表中，跳过限制检查\n", model, group)
						c.Next()
						return
					} else {
						// 如果分组没有配置模型列表（空数组），则使用分组的一般限制
						totalMaxCount = groupTotalCount
						successMaxCount = groupSuccessCount
					}
				}
				// 如果没有找到任何限制配置，使用全局默认限制
			}
		}

		// 根据存储类型选择并执行限流处理器
		if common.RedisEnabled {
			redisRateLimitHandler(duration, totalMaxCount, successMaxCount, model)(c)
		} else {
			memoryRateLimitHandler(duration, totalMaxCount, successMaxCount, model)(c)
		}
	}
}
