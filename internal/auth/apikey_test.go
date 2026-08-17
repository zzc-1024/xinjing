package auth

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"xinjing/internal/persistence/models"
)

// fakeStore 是 APIKeyStore 的内存实现，专供测试使用。
type fakeStore struct {
	keys map[string]*models.APIKey
}

func (f *fakeStore) GetByKeyHash(ctx context.Context, hash string) (*models.APIKey, error) {
	k, ok := f.keys[hash]
	if !ok {
		// 任意错误都行：AuthenticateKey 会把所有错误统一映射为 ErrInvalidKey。
		return nil, errors.New("not found")
	}
	return k, nil
}

func TestGenerateAPIKey(t *testing.T) {
	k1, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}
	k2, _ := GenerateAPIKey()
	if k1 == k2 {
		t.Errorf("两次生成的密钥不应相同")
	}
	if len(k1) < len(apiKeyPrefix) || k1[:len(apiKeyPrefix)] != apiKeyPrefix {
		t.Errorf("key %q 应带前缀 %q", k1, apiKeyPrefix)
	}
}

func TestHashAPIKeyStable(t *testing.T) {
	h1 := HashAPIKey("xj_secret")
	h2 := HashAPIKey("xj_secret")
	if h1 != h2 {
		t.Errorf("相同输入应得到相同哈希")
	}
	if len(h1) != 64 {
		t.Errorf("sha256 十六进制长度 = %d, 应为 64", len(h1))
	}
}

func TestAuthenticateKeySuccess(t *testing.T) {
	plain := "xj_test_key"
	store := &fakeStore{keys: map[string]*models.APIKey{
		HashAPIKey(plain): {
			BaseModel: models.BaseModel{ID: "key-1"},
			UserID:    "user-1",
			KeyHash:   HashAPIKey(plain),
			Scopes:    []string{"read"},
			Status:    models.APIKeyStatusActive,
		},
	}}
	p, err := AuthenticateKey(context.Background(), store, plain)
	if err != nil {
		t.Fatalf("AuthenticateKey: %v", err)
	}
	if p.UserID != "user-1" {
		t.Errorf("UserID = %q, want user-1", p.UserID)
	}
	if p.AuthMethod != AuthMethodAPIKey {
		t.Errorf("AuthMethod = %q, want apikey", p.AuthMethod)
	}
	if p.KeyID != "key-1" {
		t.Errorf("KeyID = %q, want key-1", p.KeyID)
	}
	if !p.HasScope(ScopeRead) {
		t.Errorf("应拥有 read 权限")
	}
}

func TestAuthenticateKeyNotFound(t *testing.T) {
	store := &fakeStore{keys: map[string]*models.APIKey{}}
	_, err := AuthenticateKey(context.Background(), store, "xj_missing")
	if !errors.Is(err, ErrInvalidKey) {
		t.Errorf("err = %v, want ErrInvalidKey", err)
	}
}

func TestAuthenticateKeyRevoked(t *testing.T) {
	plain := "xj_revoked"
	store := &fakeStore{keys: map[string]*models.APIKey{
		HashAPIKey(plain): {
			UserID:  "user-1",
			KeyHash: HashAPIKey(plain),
			Status:  models.APIKeyStatusRevoked,
		},
	}}
	_, err := AuthenticateKey(context.Background(), store, plain)
	if !errors.Is(err, ErrKeyRevoked) {
		t.Errorf("err = %v, want ErrKeyRevoked", err)
	}
}

func TestAuthenticateKeyExpired(t *testing.T) {
	plain := "xj_expired"
	exp := time.Now().Add(-time.Hour)
	store := &fakeStore{keys: map[string]*models.APIKey{
		HashAPIKey(plain): {
			UserID:    "user-1",
			KeyHash:   HashAPIKey(plain),
			Status:    models.APIKeyStatusActive,
			ExpiresAt: &exp,
		},
	}}
	_, err := AuthenticateKey(context.Background(), store, plain)
	if !errors.Is(err, ErrKeyExpired) {
		t.Errorf("err = %v, want ErrKeyExpired", err)
	}
}

func TestExtractAPIKey(t *testing.T) {
	r1 := httptest.NewRequest("GET", "/", nil)
	r1.Header.Set("Authorization", "Bearer xj_abc")
	if got := ExtractAPIKey(r1); got != "xj_abc" {
		t.Errorf("Bearer 形式：got %q", got)
	}

	r2 := httptest.NewRequest("GET", "/", nil)
	r2.Header.Set("X-API-Key", "xj_xyz")
	if got := ExtractAPIKey(r2); got != "xj_xyz" {
		t.Errorf("X-API-Key 形式：got %q", got)
	}

	r3 := httptest.NewRequest("GET", "/", nil)
	r3.Header.Set("Authorization", "Bearer xj_abc")
	r3.Header.Set("X-API-Key", "xj_xyz")
	if got := ExtractAPIKey(r3); got != "xj_abc" {
		t.Errorf("Authorization 应优先：got %q", got)
	}

	r4 := httptest.NewRequest("GET", "/", nil)
	if got := ExtractAPIKey(r4); got != "" {
		t.Errorf("无凭证应返回空：got %q", got)
	}
}
