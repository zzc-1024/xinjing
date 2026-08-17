package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"xinjing/internal/persistence/models"
)

// fakeRefreshStore 是 RefreshTokenStore 的内存实现。
type fakeRefreshStore struct {
	tokens map[string]*models.RefreshToken
}

func (f *fakeRefreshStore) GetByTokenHash(ctx context.Context, hash string) (*models.RefreshToken, error) {
	t, ok := f.tokens[hash]
	if !ok {
		return nil, errors.New("not found")
	}
	return t, nil
}

func (f *fakeRefreshStore) Update(ctx context.Context, token *models.RefreshToken) error {
	f.tokens[token.TokenHash] = token
	return nil
}

func TestGenerateRefreshToken(t *testing.T) {
	t1, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	t2, _ := GenerateRefreshToken()
	if t1 == t2 {
		t.Errorf("两次生成的 token 不应相同")
	}
	if len(t1) < len(refreshPrefix) || t1[:len(refreshPrefix)] != refreshPrefix {
		t.Errorf("token %q 应带前缀 %q", t1, refreshPrefix)
	}
}

func TestHashRefreshTokenStable(t *testing.T) {
	h1 := HashRefreshToken("rt_secret")
	h2 := HashRefreshToken("rt_secret")
	if h1 != h2 {
		t.Errorf("相同输入应得到相同哈希")
	}
	if len(h1) != 64 {
		t.Errorf("sha256 十六进制长度 = %d, 应为 64", len(h1))
	}
}

func TestValidateRefreshTokenSuccess(t *testing.T) {
	plain := "rt_valid"
	exp := time.Now().Add(time.Hour)
	store := &fakeRefreshStore{tokens: map[string]*models.RefreshToken{
		HashRefreshToken(plain): {
			BaseModel: models.BaseModel{ID: "rt-1"},
			UserID:    "user-1",
			TokenHash: HashRefreshToken(plain),
			Scopes:    []string{"read"},
			ExpiresAt: exp,
		},
	}}
	tok, err := ValidateRefreshToken(context.Background(), store, plain)
	if err != nil {
		t.Fatalf("ValidateRefreshToken: %v", err)
	}
	if tok.UserID != "user-1" {
		t.Errorf("UserID = %q, want user-1", tok.UserID)
	}
}

func TestValidateRefreshTokenNotFound(t *testing.T) {
	store := &fakeRefreshStore{tokens: map[string]*models.RefreshToken{}}
	if _, err := ValidateRefreshToken(context.Background(), store, "rt_missing"); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Errorf("err = %v, want ErrInvalidRefreshToken", err)
	}
}

func TestValidateRefreshTokenRevoked(t *testing.T) {
	plain := "rt_revoked"
	now := time.Now()
	store := &fakeRefreshStore{tokens: map[string]*models.RefreshToken{
		HashRefreshToken(plain): {
			UserID:    "user-1",
			TokenHash: HashRefreshToken(plain),
			ExpiresAt: time.Now().Add(time.Hour),
			RevokedAt: &now,
		},
	}}
	if _, err := ValidateRefreshToken(context.Background(), store, plain); !errors.Is(err, ErrRefreshTokenRevoked) {
		t.Errorf("err = %v, want ErrRefreshTokenRevoked", err)
	}
}

func TestValidateRefreshTokenExpired(t *testing.T) {
	plain := "rt_expired"
	store := &fakeRefreshStore{tokens: map[string]*models.RefreshToken{
		HashRefreshToken(plain): {
			UserID:    "user-1",
			TokenHash: HashRefreshToken(plain),
			ExpiresAt: time.Now().Add(-time.Hour),
		},
	}}
	if _, err := ValidateRefreshToken(context.Background(), store, plain); !errors.Is(err, ErrRefreshTokenExpired) {
		t.Errorf("err = %v, want ErrRefreshTokenExpired", err)
	}
}
