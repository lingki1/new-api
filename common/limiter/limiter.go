package limiter

import (
	"context"
	_ "embed"
	"fmt"
	"github.com/go-redis/redis/v8"
	"one-api/common"
	"sync"
	"time"
)

//go:embed lua/rate_limit.lua
var rateLimitScript string

//go:embed lua/sliding_window_rate_limit.lua
var slidingWindowRateLimitScript string

type RedisLimiter struct {
	client                       *redis.Client
	limitScriptSHA               string
	slidingWindowLimitScriptSHA  string
}

var (
	instance *RedisLimiter
	once     sync.Once
)

func New(ctx context.Context, r *redis.Client) *RedisLimiter {
	once.Do(func() {
		// 预加载脚本
		limitSHA, err := r.ScriptLoad(ctx, rateLimitScript).Result()
		if err != nil {
			common.SysLog(fmt.Sprintf("Failed to load rate limit script: %v", err))
		}
		
		slidingWindowSHA, err := r.ScriptLoad(ctx, slidingWindowRateLimitScript).Result()
		if err != nil {
			common.SysLog(fmt.Sprintf("Failed to load sliding window rate limit script: %v", err))
		}
		
		instance = &RedisLimiter{
			client:                      r,
			limitScriptSHA:              limitSHA,
			slidingWindowLimitScriptSHA: slidingWindowSHA,
		}
	})

	return instance
}

// SlidingWindowAllow 滑动窗口限流检查和记录
func (rl *RedisLimiter) SlidingWindowAllow(ctx context.Context, key string, maxCount int, duration int64, shouldRecord bool) (bool, error) {
	currentTime := time.Now().Unix()
	recordFlag := 0
	if shouldRecord {
		recordFlag = 1
	}

	// 执行滑动窗口限流脚本
	result, err := rl.client.EvalSha(
		ctx,
		rl.slidingWindowLimitScriptSHA,
		[]string{key},
		maxCount,
		duration,
		currentTime,
		recordFlag,
	).Int()

	if err != nil {
		return false, fmt.Errorf("sliding window rate limit failed: %w", err)
	}
	
	return result == 1, nil
}

// SlidingWindowCheck 仅检查限流，不记录请求
func (rl *RedisLimiter) SlidingWindowCheck(ctx context.Context, key string, maxCount int, duration int64) (bool, error) {
	return rl.SlidingWindowAllow(ctx, key, maxCount, duration, false)
}

// SlidingWindowRecord 检查并记录请求
func (rl *RedisLimiter) SlidingWindowRecord(ctx context.Context, key string, maxCount int, duration int64) (bool, error) {
	return rl.SlidingWindowAllow(ctx, key, maxCount, duration, true)
}

func (rl *RedisLimiter) Allow(ctx context.Context, key string, opts ...Option) (bool, error) {
	// 默认配置
	config := &Config{
		Capacity:  10,
		Rate:      1,
		Requested: 1,
	}

	// 应用选项模式
	for _, opt := range opts {
		opt(config)
	}

	// 执行限流
	result, err := rl.client.EvalSha(
		ctx,
		rl.limitScriptSHA,
		[]string{key},
		config.Requested,
		config.Rate,
		config.Capacity,
	).Int()

	if err != nil {
		return false, fmt.Errorf("rate limit failed: %w", err)
	}
	return result == 1, nil
}

// Config 配置选项模式
type Config struct {
	Capacity  int64
	Rate      int64
	Requested int64
}

type Option func(*Config)

func WithCapacity(c int64) Option {
	return func(cfg *Config) { cfg.Capacity = c }
}

func WithRate(r int64) Option {
	return func(cfg *Config) { cfg.Rate = r }
}

func WithRequested(n int64) Option {
	return func(cfg *Config) { cfg.Requested = n }
}
