package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"xinjing/internal/auth"
)

// fakeAuthenticator 是 auth.Authenticator 的假实现，用于隔离测试中间件本身的行为。
type fakeAuthenticator struct {
	principal auth.Principal
	err       error
}

func (f *fakeAuthenticator) Authenticate(ctx context.Context, r *http.Request) (auth.Principal, error) {
	return f.principal, f.err
}

func TestAuthenticateInjectsPrincipal(t *testing.T) {
	want := auth.Principal{UserID: "u1", AuthMethod: auth.AuthMethodJWT, Scopes: []string{"read"}}
	authMw := Authenticate(&fakeAuthenticator{principal: want})

	var got auth.Principal
	ok := false
	h := authMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok = auth.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if !ok {
		t.Fatalf("Principal 未注入上下文")
	}
	if got.UserID != "u1" {
		t.Errorf("UserID = %q, want u1", got.UserID)
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestAuthenticateMissingCredentials(t *testing.T) {
	authMw := Authenticate(&fakeAuthenticator{err: auth.ErrMissingCredentials})
	called := false
	h := authMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if called {
		t.Errorf("认证失败时不应调用后续 handler")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAuthenticateInvalidCredentials(t *testing.T) {
	authMw := Authenticate(&fakeAuthenticator{err: auth.ErrInvalidToken})
	h := authMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestRequireScopeAllowed(t *testing.T) {
	scopeMw := RequireScope(auth.ScopeRead)
	called := false
	h := scopeMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "u1", Scopes: []string{"read"}})
	r := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if !called {
		t.Errorf("有权限时应调用 handler")
	}
}

func TestRequireScopeForbidden(t *testing.T) {
	scopeMw := RequireScope(auth.ScopeAdmin)
	called := false
	h := scopeMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "u1", Scopes: []string{"read"}})
	r := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if called {
		t.Errorf("无权限时不应调用 handler")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestRequireScopeUnauthenticated(t *testing.T) {
	scopeMw := RequireScope(auth.ScopeRead)
	h := scopeMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	// 上下文里没有 Principal（未经过 Authenticate 中间件）
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}
