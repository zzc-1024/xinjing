package ratelimit

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"xinjing/internal/persistence/cache"
)

// state 是令牌桶的持久化状态，序列化后存入缓存。
type state struct {
	Tokens float64   `json:"tokens"` // 当前令牌数
	Last   time.Time `json:"last"`   // 上次补充时间
}

// TokenBucket 是基于 Cache 的令牌桶限流器。
//
// 惰性补充：不启动后台定时器，只在每次请求时计算「距上次补充过去了多久，
// 应该补多少令牌」。省资源且天然支持不活跃 key 不耗 CPU。
//
// 并发：单机开发下用互斥锁串行化「读缓存 → 计算 → 写回」；
// 未来 valkey 后端会用原子 Lua 脚本替换，本实现作为「单机可跑的版本」。
type TokenBucket struct {
	cache cache.Cache
	mu    sync.Mutex
	now   func() time.Time
}

// NewTokenBucket 创建令牌桶限流器。cache 通常是 memory（开发）/ valkey（生产）。
func NewTokenBucket(c cache.Cache) *TokenBucket {
	return &TokenBucket{cache: c, now: time.Now}
}

// Allow 判断 key 在 policy 下是否允许本次请求。
// 返回 true 表示放行（消耗一个令牌）；false 表示被限流。
// 返回的 error 仅表示「限流器自身故障」（如缓存不可用），与是否限流无关。
func (t *TokenBucket) Allow(ctx context.Context, key string, p Policy) (bool, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	ckey := rateLimitKeyPrefix + string(p.Scope) + ":" + key

	// 1. 读当前状态；缓存未命中则按「满桶 + 当前时刻」初始化
	st := state{Tokens: float64(p.Burst), Last: t.now()}
	if raw, err := t.cache.Get(ctx, ckey); err == nil {
		if err := json.Unmarshal(raw, &st); err != nil {
			// 数据损坏：按满桶重置，继续放行本次请求
			st = state{Tokens: float64(p.Burst), Last: t.now()}
		}
	} else if err != cache.ErrMiss {
		// 缓存故障：fail-open，放行并上报错误
		return true, err
	}

	// 2. 惰性补充：按速率补令牌，上限为桶容量 Burst
	elapsed := t.now().Sub(st.Last).Seconds()
	refill := elapsed * p.refillPerSecond()
	st.Tokens = minFloat(p.Burst, st.Tokens+refill)

	// 3. 消耗一个令牌（不足则拒绝）
	allowed := st.Tokens >= 1
	if allowed {
		st.Tokens -= 1
	}

	// 4. 写回状态（无论放行还是拒绝都要更新 last，防止「拒绝期间不补令牌」）
	st.Last = t.now()
	raw, _ := json.Marshal(st)
	_ = t.cache.Set(ctx, ckey, raw, ttlFor(p))

	return allowed, nil
}

// ttlFor 返回缓存条目的过期时间：至少一个窗口 + 宽裕量，让不活跃 key 自动清理。
func ttlFor(p Policy) time.Duration {
	window := time.Duration(p.WindowSec) * time.Second
	if window <= 0 {
		window = time.Minute
	}
	return window + time.Minute
}

// minFloat 返回 int 上限与 float64 值的较小者。
func minFloat(limit int, v float64) float64 {
	if float64(limit) < v {
		return float64(limit)
	}
	return v
}
