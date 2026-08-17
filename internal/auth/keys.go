package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
)

// RSA 密钥解析失败时返回的错误。
var (
	// ErrInvalidPrivateKey 表示私钥 PEM 内容非法或不是 RSA 私钥。
	ErrInvalidPrivateKey = errors.New("invalid rsa private key")
	// ErrInvalidPublicKey 表示公钥 PEM 内容非法或不是 RSA 公钥。
	ErrInvalidPublicKey = errors.New("invalid rsa public key")
)

// GenerateRSAKeyPair 生成一个新的 RSA 密钥对。
// bits 是模数位数：2048 是当前主流安全下限，4096 更安全但签名更慢。
// 私钥只应留在签发节点；公钥可安全分发给所有验证节点/第三方。
func GenerateRSAKeyPair(bits int) (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, bits)
}

// MarshalRSAPrivateKeyPEM 把私钥导出为 PKCS#1 的 PEM 文本（-----BEGIN RSA PRIVATE KEY-----）。
func MarshalRSAPrivateKeyPEM(key *rsa.PrivateKey) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
}

// MarshalRSAPublicKeyPEM 把公钥导出为 PKIX 的 PEM 文本（-----BEGIN PUBLIC KEY-----）。
func MarshalRSAPublicKeyPEM(key *rsa.PublicKey) []byte {
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return nil
	}
	return pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: der,
	})
}

// ParseRSAPrivateKeyPEM 解析私钥 PEM 文本，兼容两种常见格式：
//   - PKCS#1：-----BEGIN RSA PRIVATE KEY-----
//   - PKCS#8：-----BEGIN PRIVATE KEY-----（openssl 默认导出的就是这种）
func ParseRSAPrivateKeyPEM(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, ErrInvalidPrivateKey
	}
	// 先按 PKCS#1 尝试
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	// 再按 PKCS#8 尝试
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, ErrInvalidPrivateKey
		}
		return rsaKey, nil
	}
	return nil, ErrInvalidPrivateKey
}

// ParseRSAPublicKeyPEM 解析公钥 PEM 文本，兼容：
//   - PKIX：-----BEGIN PUBLIC KEY-----
//   - PKCS#1：-----BEGIN RSA PUBLIC KEY-----
func ParseRSAPublicKeyPEM(data []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, ErrInvalidPublicKey
	}
	if key, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		rsaKey, ok := key.(*rsa.PublicKey)
		if !ok {
			return nil, ErrInvalidPublicKey
		}
		return rsaKey, nil
	}
	if key, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, ErrInvalidPublicKey
}
