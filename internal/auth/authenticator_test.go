package auth

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"
)

func TestJWTAuthenticatorSuccess(t *testing.T) {
	m := NewJWTManager(testRSAKey, &testRSAKey.PublicKey)
	token, _ := m.Issue(context.Background(), "u1", []string{"read"}, time.Hour)
	a := &JWTAuthenticator{Manager: m}
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer "+token)

	p, err := a.Authenticate(context.Background(), r)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if p.UserID != "u1" {
		t.Errorf("UserID = %q, want u1", p.UserID)
	}
}

func TestJWTAuthenticatorMissing(t *testing.T) {
	a := &JWTAuthenticator{Manager: NewJWTManager(testRSAKey, &testRSAKey.PublicKey)}
	r := httptest.NewRequest("GET", "/", nil)
	if _, err := a.Authenticate(context.Background(), r); !errors.Is(err, ErrMissingCredentials) {
		t.Errorf("err = %v, want ErrMissingCredentials", err)
	}
}

func TestExtractBearerToken(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer abc.def.ghi")
	if got := ExtractBearerToken(r); got != "abc.def.ghi" {
		t.Errorf("got %q", got)
	}

	// 大小写不敏感
	r2 := httptest.NewRequest("GET", "/", nil)
	r2.Header.Set("Authorization", "bearer xyz")
	if got := ExtractBearerToken(r2); got != "xyz" {
		t.Errorf("got %q", got)
	}

	// 格式不对（非两段式）返回空
	r3 := httptest.NewRequest("GET", "/", nil)
	r3.Header.Set("Authorization", "Bearer")
	if got := ExtractBearerToken(r3); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}
