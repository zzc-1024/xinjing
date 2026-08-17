// Package ratelimit 提供限流能力：令牌桶算法（Token Bucket）。
// 与 RateLimitPolicy 模型字段语义对齐：
//   - Burst       桶容量（突发上限，最多可积攒的令牌数）
//   - LimitCount  窗口内允许的请求数（决定补充速率）
//   - WindowSec   窗口时长（秒）
//   - Scope       限流维度（per-key / per-route / global）
package ratelimit

import "context"

// Scope 是限流维度。
type Scope string

const (
	// ScopePerKey 按调用方限流：已认证按用户 ID，未认证按客户端 IP。
	ScopePerKey Scope = "per-key"
	// ScopePerRoute 按路由路径限流。
	ScopePerRoute Scope = "per-route"
	// ScopeGlobal 全局共享一个配额。
	ScopeGlobal Scope = "global"
)

// Policy 描述一条限流策略。
type Policy struct {
	Name       string // 策略名（便于日志/审计）
	LimitCount int    // 窗口内允许的请求数
	WindowSec  int    // 窗口时长（秒）
	Burst      int    // 桶容量（突发上限）
	Scope      Scope  // 限流维度
}

// refillPerSecond 返回每秒补充的令牌数（补充速率 = LimitCount / WindowSec）。
// 令牌桶的「匀速补充」语义：把窗口配额摊到每秒，而不是窗口结束后一次性重置。
func (p Policy) refillPerSecond() float64 {
	if p.WindowSec <= 0 {
		return 0
	}
	return float64(p.LimitCount) / float64(p.WindowSec)
}

// RateLimiter 是限流器接口。业务只依赖此接口，后端实现可替换
// （当前：基于 Cache 的 TokenBucket；未来可加基于 valkey 原子脚本的实现）。
type RateLimiter interface {
	// Allow 判断 key 在 policy 下是否放行本次请求。
	// 返回 false 表示被限流；error 非空表示限流器自身故障（与是否限流无关）。
	Allow(ctx context.Context, key string, p Policy) (bool, error)
}

// rateLimitKeyPrefix 是所有限流计数 key 的统一前缀，避免与其他缓存数据冲突。
const rateLimitKeyPrefix = "rl:"
