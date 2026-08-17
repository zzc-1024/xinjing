package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"xinjing/internal/auth"
	"xinjing/internal/middleware"
)

func TestMeWithPrincipal(t *testing.T) {
	// 直接构造一个带 Principal 的上下文（等价于已通过 Authenticate 中间件）
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{
		UserID:     "user-1",
		AuthMethod: auth.AuthMethodJWT,
		Scopes:     []string{"read", "write"},
	})
	req := httptest.NewRequest(http.MethodGet, "/me", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	Me(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body meResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.UserID != "user-1" {
		t.Errorf("user_id = %q, want user-1", body.UserID)
	}
	if body.AuthMethod != string(auth.AuthMethodJWT) {
		t.Errorf("auth_method = %q, want jwt", body.AuthMethod)
	}
	if len(body.Scopes) != 2 {
		t.Errorf("scopes len = %d, want 2", len(body.Scopes))
	}
}

func TestMeWithoutPrincipal(t *testing.T) {
	// 无 Principal 的上下文 → 401
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	rec := httptest.NewRecorder()

	Me(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// TestMeFullChain 验证「JWT 认证中间件 + scope 授权 + handler」的完整链路。
// 这是方案 A 的关键端到端验证：真实 JWT → 中间件 → 授权 → /me。
func TestMeFullChain(t *testing.T) {
	privKey := newTestRSA(t)
	jwt := auth.NewJWTManager(privKey, &privKey.PublicKey)
	authenticator := &auth.JWTAuthenticator{Manager: jwt}

	// 用真实密钥签发一张带 read 权限的 JWT
	token, err := jwt.Issue(context.Background(), "user-1", []string{"read"}, 15*time.Minute)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	// 组装与 main.go 一致的路由：Chain(Me, Authenticate, RequireScope(read))
	h := middleware.Chain(
		http.HandlerFunc(Me),
		middleware.Authenticate(authenticator),
		middleware.RequireScope(auth.ScopeRead),
	)

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body meResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.UserID != "user-1" {
		t.Errorf("user_id = %q, want user-1", body.UserID)
	}
}
