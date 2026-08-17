package auth

import (
	"errors"
	"testing"
)

func TestRSAKeyPairRoundTrip(t *testing.T) {
	key, err := GenerateRSAKeyPair(2048)
	if err != nil {
		t.Fatalf("GenerateRSAKeyPair: %v", err)
	}

	// 私钥导出 → 解析 → 应一致
	privPEM := MarshalRSAPrivateKeyPEM(key)
	parsedPriv, err := ParseRSAPrivateKeyPEM(privPEM)
	if err != nil {
		t.Fatalf("ParseRSAPrivateKeyPEM: %v", err)
	}
	if !parsedPriv.Equal(key) {
		t.Errorf("私钥往返后不一致")
	}

	// 公钥导出 → 解析 → 应一致
	pubPEM := MarshalRSAPublicKeyPEM(&key.PublicKey)
	parsedPub, err := ParseRSAPublicKeyPEM(pubPEM)
	if err != nil {
		t.Fatalf("ParseRSAPublicKeyPEM: %v", err)
	}
	if !parsedPub.Equal(&key.PublicKey) {
		t.Errorf("公钥往返后不一致")
	}
}

func TestParseRSAPrivateKeyPEMInvalid(t *testing.T) {
	if _, err := ParseRSAPrivateKeyPEM([]byte("not pem")); !errors.Is(err, ErrInvalidPrivateKey) {
		t.Errorf("err = %v, want ErrInvalidPrivateKey", err)
	}
}

func TestParseRSAPublicKeyPEMInvalid(t *testing.T) {
	if _, err := ParseRSAPublicKeyPEM([]byte("not pem")); !errors.Is(err, ErrInvalidPublicKey) {
		t.Errorf("err = %v, want ErrInvalidPublicKey", err)
	}
}
