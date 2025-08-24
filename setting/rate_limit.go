package setting

import (
	"encoding/json"
	"fmt"
	"one-api/common"
	"sync"
)

var ModelRequestRateLimitEnabled = false
var ModelRequestRateLimitDurationMinutes = 1
var ModelRequestRateLimitCount = 0
var ModelRequestRateLimitSuccessCount = 1000

// 新的数据结构：支持模型特定的限制
type GroupRateLimit struct {
	TotalCount   int      `json:"total_count"`
	SuccessCount int      `json:"success_count"`
	Models       []string `json:"models,omitempty"` // 如果为空，则限制所有模型
}

var ModelRequestRateLimitGroup = map[string]GroupRateLimit{}
var ModelRequestRateLimitMutex sync.RWMutex

func ModelRequestRateLimitGroup2JSONString() string {
	ModelRequestRateLimitMutex.RLock()
	defer ModelRequestRateLimitMutex.RUnlock()

	jsonBytes, err := json.Marshal(ModelRequestRateLimitGroup)
	if err != nil {
		common.SysError("error marshalling model ratio: " + err.Error())
	}
	return string(jsonBytes)
}

func UpdateModelRequestRateLimitGroupByJSONString(jsonStr string) error {
	ModelRequestRateLimitMutex.Lock()
	defer ModelRequestRateLimitMutex.Unlock()

	// 尝试解析新的格式
	var newFormat map[string]GroupRateLimit
	err := json.Unmarshal([]byte(jsonStr), &newFormat)
	if err == nil {
		ModelRequestRateLimitGroup = newFormat
		return nil
	}

	// 如果新格式解析失败，尝试解析旧格式并转换
	var oldFormat map[string][2]int
	err = json.Unmarshal([]byte(jsonStr), &oldFormat)
	if err != nil {
		return err
	}

	// 转换为新格式
	ModelRequestRateLimitGroup = make(map[string]GroupRateLimit)
	for group, limits := range oldFormat {
		ModelRequestRateLimitGroup[group] = GroupRateLimit{
			TotalCount:   limits[0],
			SuccessCount: limits[1],
			Models:       []string{}, // 空数组表示限制所有模型
		}
	}
	return nil
}

func GetGroupRateLimit(group string) (totalCount, successCount int, found bool) {
	ModelRequestRateLimitMutex.RLock()
	defer ModelRequestRateLimitMutex.RUnlock()

	if ModelRequestRateLimitGroup == nil {
		return 0, 0, false
	}

	limits, found := ModelRequestRateLimitGroup[group]
	if !found {
		return 0, 0, false
	}
	return limits.TotalCount, limits.SuccessCount, true
}

// 新增：获取分组对特定模型的限制
func GetGroupModelRateLimit(group, model string) (totalCount, successCount int, found bool) {
	ModelRequestRateLimitMutex.RLock()
	defer ModelRequestRateLimitMutex.RUnlock()

	if ModelRequestRateLimitGroup == nil {
		return 0, 0, false
	}

	limits, found := ModelRequestRateLimitGroup[group]
	if !found {
		return 0, 0, false
	}

	// 如果没有指定模型列表，则限制所有模型
	if len(limits.Models) == 0 {
		return limits.TotalCount, limits.SuccessCount, true
	}

	// 检查模型是否在限制列表中
	for _, m := range limits.Models {
		if m == model {
			return limits.TotalCount, limits.SuccessCount, true
		}
	}

	// 模型不在限制列表中，返回未找到
	return 0, 0, false
}

func CheckModelRequestRateLimitGroup(jsonStr string) error {
	// 尝试解析新格式
	var newFormat map[string]GroupRateLimit
	err := json.Unmarshal([]byte(jsonStr), &newFormat)
	if err == nil {
		for group, limits := range newFormat {
			if limits.TotalCount < 0 || limits.SuccessCount < 1 {
				return fmt.Errorf("group %s has invalid rate limit values: total_count=%d, success_count=%d", group, limits.TotalCount, limits.SuccessCount)
			}
		}
		return nil
	}

	// 尝试解析旧格式
	var oldFormat map[string][2]int
	err = json.Unmarshal([]byte(jsonStr), &oldFormat)
	if err != nil {
		return err
	}
	for group, limits := range oldFormat {
		if limits[0] < 0 || limits[1] < 1 {
			return fmt.Errorf("group %s has negative rate limit values: [%d, %d]", group, limits[0], limits[1])
		}
	}

	return nil
}
