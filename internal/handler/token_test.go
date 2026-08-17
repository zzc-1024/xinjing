package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"xinjing/internal/auth"
	"xinjing/internal/persistence/models"
	"xinjing/internal/persistence/repo"
)

// newTestEnv 建立集成测试环境：真实 SQLite + 建表 + 注册 UUID 回调。
func newTestEnv(t *testing.T) (*gorm.DB, repo.UserRepository, repo.RefreshTokenRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/test.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := models.RegisterIDCallbacks(db); err != nil {
		t.Fatalf("register callbacks: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.RefreshToken{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db, repo.NewUserRepository(db), repo.NewRefreshTokenRepository(db)
}

// newTestRSA 生成测试用 RSA 密钥对。
func newTestRSA(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa: %v", err)
	}
	return k
}

// doToken 发送一个 JSON POST 到 /token，返回 recorder。
func doToken(h *TokenHandler, payload any) *httptest.ResponseRecorder {
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.HandleToken(rec, req)
	return rec
}

// doRefresh 发送一个 JSON POST 到 /refresh，返回 recorder。
func doRefresh(h *TokenHandler, payload any) *httptest.ResponseRecorder {
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/refresh", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.HandleRefresh(rec, req)
	return rec
}

func TestTokenAndRefreshFlow(t *testing.T) {
	_, userRepo, refreshRepo := newTestEnv(t)
	privKey := newTestRSA(t)
	jwt := auth.NewJWTManager(privKey, &privKey.PublicKey)
	h := NewTokenHandler(userRepo, refreshRepo, jwt, 15*time.Minute, 30*24*time.Hour)

	// 预置用户
	hash, _ := auth.HashPassword("s3cret")
	u := &models.User{Name: "Alice", Email: "alice@example.com", PasswordHash: hash}
	if err := userRepo.Create(context.Background(), u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	// 1) /token 密码登录
	tokenRec := doToken(h, map[string]string{
		"grant_type": "password",
		"email":      "alice@example.com",
		"password":   "s3cret",
		"scope":      "read write",
	})
	if tokenRec.Code != http.StatusOK {
		t.Fatalf("token status = %d, body = %s", tokenRec.Code, tokenRec.Body.String())
	}
	var tr tokenResponse
	if err := json.Unmarshal(tokenRec.Body.Bytes(), &tr); err != nil {
		t.Fatalf("decode token: %v", err)
	}
	if tr.AccessToken == "" || tr.RefreshToken == "" {
		t.Fatalf("空 token: %+v", tr)
	}
	if tr.ExpiresIn != 900 {
		t.Errorf("expires_in = %d, want 900", tr.ExpiresIn)
	}
	if tr.TokenType != "Bearer" {
		t.Errorf("token_type = %q, want Bearer", tr.TokenType)
	}

	// 2) 验证 access token 的 JWT 内容
	p, err := jwt.Verify(context.Background(), tr.AccessToken)
	if err != nil {
		t.Fatalf("verify access: %v", err)
	}
	if p.UserID != u.ID {
		t.Errorf("access user_id = %q, want %q", p.UserID, u.ID)
	}
	if !p.HasScope(auth.ScopeRead) || !p.HasScope(auth.ScopeWrite) {
		t.Errorf("access 应含 read/write 权限: %v", p.Scopes)
	}

	// 3) /refresh 用旧 refresh token 换新的
	refreshRec := doRefresh(h, map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": tr.RefreshToken,
	})
	if refreshRec.Code != http.StatusOK {
		t.Fatalf("refresh status = %d, body = %s", refreshRec.Code, refreshRec.Body.String())
	}
	var rr tokenResponse
	if err := json.Unmarshal(refreshRec.Body.Bytes(), &rr); err != nil {
		t.Fatalf("decode refresh: %v", err)
	}
	if rr.RefreshToken == tr.RefreshToken {
		t.Errorf("rotate 后 refresh token 应更换")
	}
	if rr.AccessToken == "" {
		t.Errorf("refresh 应返回新 access token")
	}

	// 4) 旧 refresh token 已被吊销，复用应 401
	oldRec := doRefresh(h, map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": tr.RefreshToken,
	})
	if oldRec.Code != http.StatusUnauthorized {
		t.Errorf("复用已旋转的旧 refresh token 应 401, got %d", oldRec.Code)
	}
}

func TestTokenWrongPassword(t *testing.T) {
	_, userRepo, refreshRepo := newTestEnv(t)
	privKey := newTestRSA(t)
	h := NewTokenHandler(userRepo, refreshRepo, auth.NewJWTManager(privKey, &privKey.PublicKey), 15*time.Minute, 30*24*time.Hour)

	hash, _ := auth.HashPassword("right")
	u := &models.User{Name: "Bob", Email: "bob@example.com", PasswordHash: hash}
	_ = userRepo.Create(context.Background(), u)

	rec := doToken(h, map[string]string{
		"grant_type": "password",
		"email":      "bob@example.com",
		"password":   "wrong",
		"scope":      "read",
	})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("错误密码应 401, got %d", rec.Code)
	}
}

func TestTokenMissingGrantType(t *testing.T) {
	_, userRepo, refreshRepo := newTestEnv(t)
	privKey := newTestRSA(t)
	h := NewTokenHandler(userRepo, refreshRepo, auth.NewJWTManager(privKey, &privKey.PublicKey), 15*time.Minute, 30*24*time.Hour)

	rec := doToken(h, map[string]string{
		"email":    "x@example.com",
		"password": "y",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("缺 grant_type 应 400, got %d", rec.Code)
	}
}

// doRevoke 发送一个 JSON POST 到 /revoke，返回 recorder。
func doRevoke(h *TokenHandler, payload any) *httptest.ResponseRecorder {
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/revoke", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.HandleRevoke(rec, req)
	return rec
}

func TestRevoke(t *testing.T) {
	_, userRepo, refreshRepo := newTestEnv(t)
	privKey := newTestRSA(t)
	h := NewTokenHandler(userRepo, refreshRepo, auth.NewJWTManager(privKey, &privKey.PublicKey), 15*time.Minute, 30*24*time.Hour)

	// 先登录拿到 refresh token
	hash, _ := auth.HashPassword("s3cret")
	u := &models.User{Name: "Carol", Email: "carol@example.com", PasswordHash: hash}
	_ = userRepo.Create(context.Background(), u)

	tokenRec := doToken(h, map[string]string{
		"grant_type": "password",
		"email":      "carol@example.com",
		"password":   "s3cret",
		"scope":      "read",
	})
	var tr tokenResponse
	_ = json.Unmarshal(tokenRec.Body.Bytes(), &tr)

	// 1) 吊销该 refresh token → 200
	revokeRec := doRevoke(h, map[string]string{"refresh_token": tr.RefreshToken})
	if revokeRec.Code != http.StatusOK {
		t.Fatalf("revoke status = %d, body = %s", revokeRec.Code, revokeRec.Body.String())
	}

	// 2) 吊销后该 refresh token 无法再兑换 → 401
	refreshRec := doRefresh(h, map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": tr.RefreshToken,
	})
	if refreshRec.Code != http.StatusUnauthorized {
		t.Errorf("吊销后 refresh 应 401, got %d", refreshRec.Code)
	}

	// 3) 重复吊销同一 token → 幂等，仍 200
	revokeAgain := doRevoke(h, map[string]string{"refresh_token": tr.RefreshToken})
	if revokeAgain.Code != http.StatusOK {
		t.Errorf("重复吊销应幂等 200, got %d", revokeAgain.Code)
	}
}

func TestRevokeMissingToken(t *testing.T) {
	_, userRepo, refreshRepo := newTestEnv(t)
	privKey := newTestRSA(t)
	h := NewTokenHandler(userRepo, refreshRepo, auth.NewJWTManager(privKey, &privKey.PublicKey), 15*time.Minute, 30*24*time.Hour)

	rec := doRevoke(h, map[string]string{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("缺 refresh_token 应 400, got %d", rec.Code)
	}
}
