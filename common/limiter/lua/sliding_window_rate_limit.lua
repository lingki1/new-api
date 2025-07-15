-- 滑动窗口限流器
-- KEYS[1]: 限流器唯一标识
-- ARGV[1]: 最大请求数
-- ARGV[2]: 时间窗口（秒）
-- ARGV[3]: 当前时间戳（秒）
-- ARGV[4]: 是否记录请求（1=记录，0=仅检查）

local key = KEYS[1]
local maxCount = tonumber(ARGV[1])
local duration = tonumber(ARGV[2])
local currentTime = tonumber(ARGV[3])
local shouldRecord = tonumber(ARGV[4])

-- 如果maxCount为0，表示不限制
if maxCount == 0 then
    return 1
end

-- 计算时间窗口开始时间
local windowStart = currentTime - duration

-- 清理过期的时间戳
redis.call('ZREMRANGEBYSCORE', key, 0, windowStart)

-- 获取当前窗口内的请求数
local currentCount = redis.call('ZCARD', key)

-- 检查是否超过限制
if currentCount >= maxCount then
    return 0
end

-- 如果需要记录请求，添加新的时间戳
if shouldRecord == 1 then
    -- 使用当前时间戳作为score和member，确保唯一性
    local member = currentTime .. ':' .. math.random(1000000)
    redis.call('ZADD', key, currentTime, member)
    
    -- 设置过期时间，稍长于时间窗口
    redis.call('EXPIRE', key, duration + 60)
end

return 1 