package gateway

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"xinjing/internal/auth"
	"xinjing/internal/persistence/cache"
	"xinjing/internal/ratelimit"
)

const ttl = time.Hour

// testRSAKey 生成测试用 RSA 密钥。
func testRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa: %v", err)
	}
	return k
}

// okHandler 返回 200 的普通处理器。
func okHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func TestRegistryMountsWithAuthAndScope(t *testing.T) {
	privKey := testRSAKey(t)
	jwt := auth.NewJWTManager(privKey, &privKey.PublicKey)
	authenticator := &auth.JWTAuthenticator{Manager: jwt}
	limiter := ratelimit.NewTokenBucket(cache.NewMemory())

	reg := NewRegistry(authenticator, limiter)
	reg.Add(Route{Method: "GET", Pattern: "/me", Handler: okHandler, Scope: auth.ScopeRead})

	mux := http.NewServeMux()
	reg.Mount(mux)

	// 无 token → 401
	r1 := httptest.NewRequest("GET", "/me", nil)
	w1 := httptest.NewRecorder()
	mux.ServeHTTP(w1, r1)
	if w1.Code != http.StatusUnauthorized {
		t.Fatalf("无 token 应 401, got %d", w1.Code)
	}

	// 有合法 token 但无 read scope → 403
	tokenNoScope, _ := jwt.Issue(context.Background(), "u1", nil, ttl)
	r2 := httptest.NewRequest("GET", "/me", nil)
	r2.Header.Set("Authorization", "Bearer "+tokenNoScope)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, r2)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("无权限应 403, got %d", w2.Code)
	}

	// 有 read scope → 200
	tokenRead, _ := jwt.Issue(context.Background(), "u1", []string{"read"}, ttl)
	r3 := httptest.NewRequest("GET", "/me", nil)
	r3.Header.Set("Authorization", "Bearer "+tokenRead)
	w3 := httptest.NewRecorder()
	mux.ServeHTTP(w3, r3)
	if w3.Code != http.StatusOK {
		t.Fatalf("有权限应 200, got %d", w3.Code)
	}
}

func TestRegistryNoScopeRoute(t *testing.T) {
	privKey := testRSAKey(t)
	jwt := auth.NewJWTManager(privKey, &privKey.PublicKey)
	authenticator := &auth.JWTAuthenticator{Manager: jwt}
	limiter := ratelimit.NewTokenBucket(cache.NewMemory())

	reg := NewRegistry(authenticator, limiter)
	reg.Add(Route{Method: "GET", Pattern: "/plain", Handler: okHandler}) // 无 Scope

	mux := http.NewServeMux()
	reg.Mount(mux)

	token, _ := jwt.Issue(context.Background(), "u1", nil, ttl)
	r := httptest.NewRequest("GET", "/plain", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("无 scope 要求的路由应 200, got %d", w.Code)
	}
}

func TestSubRouterInheritsPrefixAndScope(t *testing.T) {
	privKey := testRSAKey(t)
	jwt := auth.NewJWTManager(privKey, &privKey.PublicKey)
	authenticator := &auth.JWTAuthenticator{Manager: jwt}
	limiter := ratelimit.NewTokenBucket(cache.NewMemory())

	reg := NewRegistry(authenticator, limiter)

	// 子路由：前缀 /api，默认 scope=write
	sub := reg.Sub("/api", auth.ScopeWrite, nil)
	sub.Add(Route{Method: "GET", Pattern: "/list", Handler: okHandler})    // 实际路径 /api/list，继承 write scope
	sub.Add(Route{Method: "POST", Pattern: "/create", Handler: okHandler}) // 实际路径 /api/create

	mux := http.NewServeMux()
	reg.Mount(mux)

	// 用 write scope 的 token 访问 /api/list → 200
	tokenWrite, _ := jwt.Issue(context.Background(), "u1", []string{"write"}, ttl)
	r := httptest.NewRequest("GET", "/api/list", nil)
	r.Header.Set("Authorization", "Bearer "+tokenWrite)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("/api/list 应 200 (write scope), got %d", w.Code)
	}

	// 用 read scope 的 token 访问 /api/list → 403（继承的是 write scope）
	tokenRead, _ := jwt.Issue(context.Background(), "u1", []string{"read"}, ttl)
	r2 := httptest.NewRequest("GET", "/api/list", nil)
	r2.Header.Set("Authorization", "Bearer "+tokenRead)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, r2)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("/api/list 应 403 (read 无 write 权限), got %d", w2.Code)
	}

	// 访问未注册的 /api/other → 404
	r3 := httptest.NewRequest("GET", "/api/other", nil)
	r3.Header.Set("Authorization", "Bearer "+tokenWrite)
	w3 := httptest.NewRecorder()
	mux.ServeHTTP(w3, r3)
	if w3.Code != http.StatusNotFound {
		t.Fatalf("/api/other 应 404, got %d", w3.Code)
	}
}

func TestSubRouterOverrideScope(t *testing.T) {
	privKey := testRSAKey(t)
	jwt := auth.NewJWTManager(privKey, &privKey.PublicKey)
	authenticator := &auth.JWTAuthenticator{Manager: jwt}
	limiter := ratelimit.NewTokenBucket(cache.NewMemory())

	reg := NewRegistry(authenticator, limiter)
	sub := reg.Sub("/api", auth.ScopeWrite, nil)

	// 显式覆盖本条路由的 scope 为 read（覆盖子路由默认的 write）
	sub.Add(Route{Method: "GET", Pattern: "/readonly", Handler: okHandler, Scope: auth.ScopeRead})

	mux := http.NewServeMux()
	reg.Mount(mux)

	// read scope 的 token 应能访问（覆盖生效）
	tokenRead, _ := jwt.Issue(context.Background(), "u1", []string{"read"}, ttl)
	r := httptest.NewRequest("GET", "/api/readonly", nil)
	r.Header.Set("Authorization", "Bearer "+tokenRead)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("/api/readonly 应 200 (覆盖为 read), got %d", w.Code)
	}
}

func TestRegistryRateLimit(t *testing.T) {
	privKey := testRSAKey(t)
	jwt := auth.NewJWTManager(privKey, &privKey.PublicKey)
	authenticator := &auth.JWTAuthenticator{Manager: jwt}
	limiter := ratelimit.NewTokenBucket(cache.NewMemory())

	policy := &ratelimit.Policy{LimitCount: 10, WindowSec: 60, Burst: 1, Scope: ratelimit.ScopePerKey}
	reg := NewRegistry(authenticator, limiter)
	reg.Add(Route{Method: "GET", Pattern: "/limited", Handler: okHandler, Policy: policy})

	mux := http.NewServeMux()
	reg.Mount(mux)

	token, _ := jwt.Issue(context.Background(), "u1", nil, ttl)
	do := func() int {
		r := httptest.NewRequest("GET", "/limited", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w.Code
	}

	if do() != http.StatusOK {
		t.Fatalf("首个请求应 200")
	}
	if do() != http.StatusTooManyRequests {
		t.Fatalf("burst=1 后第二个请求应 429")
	}
}

func TestRegistryInclude(t *testing.T) {
	privKey := testRSAKey(t)
	jwt := auth.NewJWTManager(privKey, &privKey.PublicKey)
	authenticator := &auth.JWTAuthenticator{Manager: jwt}
	limiter := ratelimit.NewTokenBucket(cache.NewMemory())

	// 一个独立的子 Registry：/functions 相关路由
	functions := NewRegistry(authenticator, limiter)
	functions.Add(Route{Method: "GET", Pattern: "/list", Handler: okHandler, Scope: auth.ScopeRead})
	functions.Add(Route{Method: "POST", Pattern: "/create", Handler: okHandler, Scope: auth.ScopeWrite})

	// 主 Registry：把 functions 挂到 /api 前缀下
	main := NewRegistry(authenticator, limiter)
	main.Include("/api", functions)

	mux := http.NewServeMux()
	main.Mount(mux)

	// /api/list 需要 read
	tokenRead, _ := jwt.Issue(context.Background(), "u1", []string{"read"}, ttl)
	r1 := httptest.NewRequest("GET", "/api/list", nil)
	r1.Header.Set("Authorization", "Bearer "+tokenRead)
	w1 := httptest.NewRecorder()
	mux.ServeHTTP(w1, r1)
	if w1.Code != http.StatusOK {
		t.Fatalf("/api/list 应 200 (read), got %d", w1.Code)
	}

	// /api/create 需要 write，read token 应 403
	r2 := httptest.NewRequest("POST", "/api/create", nil)
	r2.Header.Set("Authorization", "Bearer "+tokenRead)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, r2)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("/api/create 应 403 (read 无 write), got %d", w2.Code)
	}

	// write token 访问 /api/create 应 200
	tokenWrite, _ := jwt.Issue(context.Background(), "u1", []string{"write"}, ttl)
	r3 := httptest.NewRequest("POST", "/api/create", nil)
	r3.Header.Set("Authorization", "Bearer "+tokenWrite)
	w3 := httptest.NewRecorder()
	mux.ServeHTTP(w3, r3)
	if w3.Code != http.StatusOK {
		t.Fatalf("/api/create 应 200 (write), got %d", w3.Code)
	}

	// 无 token 应 401（Include 后的路由仍统一走认证）
	r4 := httptest.NewRequest("GET", "/api/list", nil)
	w4 := httptest.NewRecorder()
	mux.ServeHTTP(w4, r4)
	if w4.Code != http.StatusUnauthorized {
		t.Fatalf("/api/list 无 token 应 401, got %d", w4.Code)
	}
}

func TestRegistryIncludeNil(t *testing.T) {
	privKey := testRSAKey(t)
	jwt := auth.NewJWTManager(privKey, &privKey.PublicKey)
	limiter := ratelimit.NewTokenBucket(cache.NewMemory())
	main := NewRegistry(&auth.JWTAuthenticator{Manager: jwt}, limiter)

	// Include nil 不应 panic，也不产生任何路由
	main.Include("/api", nil)
	if len(main.routes) != 0 {
		t.Fatalf("Include nil 后路由数 = %d, want 0", len(main.routes))
	}
}
