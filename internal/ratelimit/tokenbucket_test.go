package ratelimit

import (
	"context"
	"testing"
	"time"

	"xinjing/internal/persistence/cache"
)

// newTestLimiter 创建一个带可注入时钟的令牌桶（便于模拟时间流逝）。
func newTestLimiter() (*TokenBucket, *fakeClock, cache.Cache) {
	c := cache.NewMemory()
	clk := &fakeClock{now: time.Now()}
	tb := NewTokenBucket(c)
	tb.now = clk.Now
	return tb, clk, c
}

// fakeClock 是可手动拨动的时钟。
type fakeClock struct {
	now time.Time
}

func (f *fakeClock) Now() time.Time { return f.now }

// advance 把时钟向前拨 d。
func (f *fakeClock) advance(d time.Duration) { f.now = f.now.Add(d) }

func TestAllowWithinBurst(t *testing.T) {
	tb, _, _ := newTestLimiter()
	p := Policy{LimitCount: 10, WindowSec: 60, Burst: 3, Scope: ScopePerKey}

	// 桶容量 3：前 3 个请求都放行
	for i := 0; i < 3; i++ {
		ok, err := tb.Allow(context.Background(), "user-1", p)
		if err != nil {
			t.Fatalf("Allow #%d: %v", i+1, err)
		}
		if !ok {
			t.Fatalf("第 %d 个请求应放行（burst=3）", i+1)
		}
	}
	// 第 4 个：桶空，拒绝
	ok, _ := tb.Allow(context.Background(), "user-1", p)
	if ok {
		t.Errorf("第 4 个请求应被限流")
	}
}

func TestRefillAfterTime(t *testing.T) {
	tb, clk, _ := newTestLimiter()
	// 速率 = 10/60 ≈ 0.167 token/s；burst=3
	p := Policy{LimitCount: 10, WindowSec: 60, Burst: 3, Scope: ScopePerKey}

	// 耗尽桶（3 个放行 + 1 个拒绝）
	for i := 0; i < 3; i++ {
		_, _ = tb.Allow(context.Background(), "user-1", p)
	}
	if ok, _ := tb.Allow(context.Background(), "user-1", p); ok {
		t.Fatalf("桶应已空")
	}

	// 拨快 12 秒：应补充 12*0.167 ≈ 2 个令牌 → 可以放行 2 个
	clk.advance(12 * time.Second)
	for i := 0; i < 2; i++ {
		ok, err := tb.Allow(context.Background(), "user-1", p)
		if err != nil {
			t.Fatalf("refill Allow #%d: %v", i+1, err)
		}
		if !ok {
			t.Fatalf("补充后第 %d 个请求应放行", i+1)
		}
	}
	// 第 3 个（只补充了 ~2 个）应拒绝
	if ok, _ := tb.Allow(context.Background(), "user-1", p); ok {
		t.Errorf("补充额度耗尽后应拒绝")
	}
}

func TestCapAtBurst(t *testing.T) {
	tb, clk, _ := newTestLimiter()
	p := Policy{LimitCount: 1, WindowSec: 60, Burst: 5, Scope: ScopePerKey}

	// 长时间不活跃后，令牌数应被 cap 到 Burst（5），而不是无限累积
	clk.advance(10 * time.Minute) // 理论补充 600 * (1/60) = 10 个，但上限 5
	for i := 0; i < 5; i++ {
		if ok, _ := tb.Allow(context.Background(), "user-1", p); !ok {
			t.Fatalf("cap 后第 %d 个请求应放行", i+1)
		}
	}
	// 第 6 个：上限 5 已耗尽，拒绝
	if ok, _ := tb.Allow(context.Background(), "user-1", p); ok {
		t.Errorf("超过 burst 上限应拒绝")
	}
}

func TestSeparateKeysIndependent(t *testing.T) {
	tb, _, _ := newTestLimiter()
	p := Policy{LimitCount: 10, WindowSec: 60, Burst: 1, Scope: ScopePerKey}

	// 两个不同 key 各自独立：各自放行 1 个
	if ok, _ := tb.Allow(context.Background(), "user-a", p); !ok {
		t.Errorf("user-a 应放行")
	}
	if ok, _ := tb.Allow(context.Background(), "user-b", p); !ok {
		t.Errorf("user-b 应放行（独立计数）")
	}
	// 各自第 2 个都拒绝
	if ok, _ := tb.Allow(context.Background(), "user-a", p); ok {
		t.Errorf("user-a 第 2 个应拒绝")
	}
	if ok, _ := tb.Allow(context.Background(), "user-b", p); ok {
		t.Errorf("user-b 第 2 个应拒绝")
	}
}
