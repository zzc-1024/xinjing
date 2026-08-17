package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"xinjing/internal/auth"
	"xinjing/internal/persistence/cache"
	"xinjing/internal/ratelimit"
)

func TestRateLimitAllowsThenBlocks(t *testing.T) {
	limiter := ratelimit.NewTokenBucket(cache.NewMemory())
	policy := ratelimit.Policy{LimitCount: 10, WindowSec: 60, Burst: 1, Scope: ratelimit.ScopePerKey}

	h := RateLimit(limiter, policy)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 第一个请求：burst=1 放行
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "u1"})
	r1 := httptest.NewRequest(http.MethodGet, "/me", nil).WithContext(ctx)
	w1 := httptest.NewRecorder()
	h.ServeHTTP(w1, r1)
	if w1.Code != http.StatusOK {
		t.Fatalf("首个请求应放行, got %d", w1.Code)
	}

	// 第二个请求：桶空 → 429
	r2 := httptest.NewRequest(http.MethodGet, "/me", nil).WithContext(ctx)
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, r2)
	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("超限应 429, got %d", w2.Code)
	}
}

func TestRateLimitPerKeyUsesUserID(t *testing.T) {
	limiter := ratelimit.NewTokenBucket(cache.NewMemory())
	policy := ratelimit.Policy{LimitCount: 10, WindowSec: 60, Burst: 1, Scope: ratelimit.ScopePerKey}
	h := RateLimit(limiter, policy)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 不同用户各自 burst=1：都能放行一次
	for _, uid := range []string{"u1", "u2"} {
		ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: uid})
		r := httptest.NewRequest(http.MethodGet, "/me", nil).WithContext(ctx)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("用户 %s 首个请求应放行, got %d", uid, w.Code)
		}
	}
}
