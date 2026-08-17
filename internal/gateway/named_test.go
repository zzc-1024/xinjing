package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"xinjing/internal/auth"
	"xinjing/internal/persistence/cache"
	"xinjing/internal/ratelimit"
)

// countingMiddleware 返回一个「计数中间件」：每次请求时调用 counter 递增。
// 用于验证中间件是否被重复/正确执行。
func countingMiddleware(counter *int, tag string) NamedMiddleware {
	return NamedMiddleware{
		Provider: ProviderXinjing,
		Name:     tag,
		Apply: func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				*counter = *counter + 1
				next.ServeHTTP(w, r)
			})
		},
	}
}

func TestProviderValidation(t *testing.T) {
	valid := []Provider{"xinjing", "acme-corp", "foo_bar", "a1", "ABC-123_x"}
	for _, p := range valid {
		if !p.Valid() {
			t.Errorf("Provider %q 应合法", p)
		}
	}
	invalid := []Provider{"", "has.dot", "has space", "中文", "a/b"}
	for _, p := range invalid {
		if p.Valid() {
			t.Errorf("Provider %q 应非法", p)
		}
	}
}

func TestNamedMiddlewareKey(t *testing.T) {
	m := NamedMiddleware{Provider: "xinjing", Name: "auth"}
	if m.Key() != "xinjing:auth" {
		t.Errorf("Key() = %q, want xinjing:auth", m.Key())
	}
}

func TestMiddlewareRunsOncePerRoute(t *testing.T) {
	privKey := testRSAKey(t)
	jwt := auth.NewJWTManager(privKey, &privKey.PublicKey)
	authenticator := &auth.JWTAuthenticator{Manager: jwt}
	limiter := ratelimit.NewTokenBucket(cache.NewMemory())

	counter := 0
	reg := NewRegistry(authenticator, limiter)
	reg.Add(Route{
		Method:      "GET",
		Pattern:     "/tracked",
		Handler:     okHandler,
		Middlewares: []NamedMiddleware{countingMiddleware(&counter, "count")},
	})

	mux := http.NewServeMux()
	if err := reg.Mount(mux); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	token, _ := jwt.Issue(t.Context(), "u1", nil, ttl)
	r := httptest.NewRequest("GET", "/tracked", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if counter != 1 {
		t.Fatalf("中间件执行次数 = %d, want 1", counter)
	}
}

func TestDuplicateMiddlewareError(t *testing.T) {
	privKey := testRSAKey(t)
	jwt := auth.NewJWTManager(privKey, &privKey.PublicKey)
	authenticator := &auth.JWTAuthenticator{Manager: jwt}
	limiter := ratelimit.NewTokenBucket(cache.NewMemory())

	counter := 0
	reg := NewRegistry(authenticator, limiter)

	// Sub 默认带一个 xinjing:count 中间件
	sub := reg.Sub("/api", "", nil, countingMiddleware(&counter, "count"))
	// 子路由又显式带同名的 xinjing:count → 重复
	sub.Add(Route{
		Method:      "GET",
		Pattern:     "/x",
		Handler:     okHandler,
		Middlewares: []NamedMiddleware{countingMiddleware(&counter, "count")}, // 默认 ConflictError
	})

	mux := http.NewServeMux()
	if err := reg.Mount(mux); err == nil {
		t.Fatalf("重复中间件应报错, got nil")
	}
}

func TestDuplicateMiddlewareKeepFirst(t *testing.T) {
	privKey := testRSAKey(t)
	jwt := auth.NewJWTManager(privKey, &privKey.PublicKey)
	authenticator := &auth.JWTAuthenticator{Manager: jwt}
	limiter := ratelimit.NewTokenBucket(cache.NewMemory())

	firstCount := 0
	secondCount := 0

	// 两个同 Key 但有不同 Apply 的中间件，OnConflict=KeepFirst
	m1 := NamedMiddleware{Provider: "xinjing", Name: "m", Apply: func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { firstCount++; next.ServeHTTP(w, r) })
	}, OnConflict: ConflictKeepFirst}

	_ = secondCount
	// 用 m1 两次（同 Apply 即可验证「只保留一个不报错」）
	reg := NewRegistry(authenticator, limiter)
	reg.Add(Route{Method: "GET", Pattern: "/kf", Handler: okHandler, Middlewares: []NamedMiddleware{m1, m1}})

	mux := http.NewServeMux()
	if err := reg.Mount(mux); err != nil {
		t.Fatalf("KeepFirst 不应报错: %v", err)
	}

	token, _ := jwt.Issue(t.Context(), "u1", nil, ttl)
	r := httptest.NewRequest("GET", "/kf", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if firstCount != 1 {
		t.Fatalf("KeepFirst 去重后中间件应只执行 1 次, got %d", firstCount)
	}
}

func TestSubRouterScopeOverride(t *testing.T) {
	privKey := testRSAKey(t)
	jwt := auth.NewJWTManager(privKey, &privKey.PublicKey)
	authenticator := &auth.JWTAuthenticator{Manager: jwt}
	limiter := ratelimit.NewTokenBucket(cache.NewMemory())

	reg := NewRegistry(authenticator, limiter)
	sub := reg.Sub("/api", auth.ScopeRead, nil)

	// 大部分子级继承 read，其中一个显式覆盖为 write
	sub.Add(Route{Method: "GET", Pattern: "/readonly", Handler: okHandler})
	sub.Add(Route{Method: "GET", Pattern: "/writable", Handler: okHandler, Scope: auth.ScopeWrite})

	mux := http.NewServeMux()
	if err := reg.Mount(mux); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	// read token 能访问 /readonly，不能访问 /writable
	tokenRead, _ := jwt.Issue(t.Context(), "u1", []string{"read"}, ttl)
	r1 := httptest.NewRequest("GET", "/api/readonly", nil)
	r1.Header.Set("Authorization", "Bearer "+tokenRead)
	w1 := httptest.NewRecorder()
	mux.ServeHTTP(w1, r1)
	if w1.Code != http.StatusOK {
		t.Fatalf("/readonly 应 200, got %d", w1.Code)
	}

	r2 := httptest.NewRequest("GET", "/api/writable", nil)
	r2.Header.Set("Authorization", "Bearer "+tokenRead)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, r2)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("/writable 应 403 (read 无 write), got %d", w2.Code)
	}

	// write token 能访问 /writable
	tokenWrite, _ := jwt.Issue(t.Context(), "u1", []string{"write"}, ttl)
	r3 := httptest.NewRequest("GET", "/api/writable", nil)
	r3.Header.Set("Authorization", "Bearer "+tokenWrite)
	w3 := httptest.NewRecorder()
	mux.ServeHTTP(w3, r3)
	if w3.Code != http.StatusOK {
		t.Fatalf("/writable 应 200 (write), got %d", w3.Code)
	}
}

func TestIncludeWithExtraMiddleware(t *testing.T) {
	privKey := testRSAKey(t)
	jwt := auth.NewJWTManager(privKey, &privKey.PublicKey)
	authenticator := &auth.JWTAuthenticator{Manager: jwt}
	limiter := ratelimit.NewTokenBucket(cache.NewMemory())

	counter := 0

	functions := NewRegistry(authenticator, limiter)
	functions.Add(Route{Method: "GET", Pattern: "/list", Handler: okHandler})

	main := NewRegistry(authenticator, limiter)
	// Include 时额外附加一个中间件
	main.Include("/api", functions, countingMiddleware(&counter, "extra"))

	mux := http.NewServeMux()
	if err := main.Mount(mux); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	token, _ := jwt.Issue(t.Context(), "u1", nil, ttl)
	r := httptest.NewRequest("GET", "/api/list", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if counter != 1 {
		t.Fatalf("Include 附加中间件应执行 1 次, got %d", counter)
	}
}
