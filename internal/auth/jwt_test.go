package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"
)

// testRSAKey 是测试共享的密钥对。init 里生成一次并复用，
// 避免每个测试都重复生成 RSA 密钥（生成密钥对需要熵，较慢）。
var testRSAKey *rsa.PrivateKey

func init() {
	testRSAKey, _ = rsa.GenerateKey(rand.Reader, 2048)
}

func TestJWTIssueAndVerify(t *testing.T) {
	m := NewJWTManager(testRSAKey, &testRSAKey.PublicKey)
	token, err := m.Issue(context.Background(), "user-1", []string{"read"}, time.Hour)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	p, err := m.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if p.UserID != "user-1" {
		t.Errorf("UserID = %q, want user-1", p.UserID)
	}
	if p.AuthMethod != AuthMethodJWT {
		t.Errorf("AuthMethod = %q, want jwt", p.AuthMethod)
	}
	if !p.HasScope(ScopeRead) {
		t.Errorf("应拥有 read 权限")
	}
}

func TestJWTWrongPublicKey(t *testing.T) {
	// 用另一把公钥验证，签名必然失败
	otherKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	issue := NewJWTManager(testRSAKey, &testRSAKey.PublicKey)
	verify := NewJWTManager(nil, &otherKey.PublicKey)
	token, _ := issue.Issue(context.Background(), "user-1", nil, time.Hour)
	if _, err := verify.Verify(context.Background(), token); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("err = %v, want ErrInvalidToken", err)
	}
}

func TestJWTExpired(t *testing.T) {
	m := NewJWTManager(testRSAKey, &testRSAKey.PublicKey)
	token, err := m.Issue(context.Background(), "user-1", nil, -time.Hour)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := m.Verify(context.Background(), token); !errors.Is(err, ErrTokenExpired) {
		t.Errorf("err = %v, want ErrTokenExpired", err)
	}
}

func TestJWTTampered(t *testing.T) {
	m := NewJWTManager(testRSAKey, &testRSAKey.PublicKey)
	token, _ := m.Issue(context.Background(), "user-1", nil, time.Hour)
	tampered := token[:len(token)-1] + "x"
	if _, err := m.Verify(context.Background(), tampered); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("err = %v, want ErrInvalidToken", err)
	}
}

func TestJWTVerifyOnlyWithPublicKey(t *testing.T) {
	// 纯验证节点：只持有公钥，能 Verify 但不能 Issue
	m := NewJWTManager(nil, &testRSAKey.PublicKey)
	token, _ := NewJWTManager(testRSAKey, &testRSAKey.PublicKey).Issue(context.Background(), "user-1", nil, time.Hour)
	if _, err := m.Verify(context.Background(), token); err != nil {
		t.Errorf("仅凭公钥验证应成功: %v", err)
	}
	if _, err := m.Issue(context.Background(), "user-1", nil, time.Hour); !errors.Is(err, ErrMissingKey) {
		t.Errorf("无私钥签发应返回 ErrMissingKey, got %v", err)
	}
}
