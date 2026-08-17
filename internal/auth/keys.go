package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
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

// LoadJWTManager 从 PEM 文件路径加载 RSA 密钥并构建 JWTManager。
// 空路径表示缺少对应密钥（对应能力在 Issue/Verify 时返回 ErrMissingKey）。
// 由调用方决定私钥/公钥是否必须：认证服务必须配私钥（签发），网关必须配公钥（验签）。
func LoadJWTManager(privateKeyPath, publicKeyPath string) (*JWTManager, error) {
	var priv *rsa.PrivateKey
	var pub *rsa.PublicKey
	var err error

	if privateKeyPath != "" {
		data, readErr := os.ReadFile(privateKeyPath)
		if readErr != nil {
			return nil, fmt.Errorf("read private key %s: %w", privateKeyPath, readErr)
		}
		if priv, err = ParseRSAPrivateKeyPEM(data); err != nil {
			return nil, fmt.Errorf("parse private key %s: %w", privateKeyPath, err)
		}
	}

	if publicKeyPath != "" {
		data, readErr := os.ReadFile(publicKeyPath)
		if readErr != nil {
			return nil, fmt.Errorf("read public key %s: %w", publicKeyPath, readErr)
		}
		if pub, err = ParseRSAPublicKeyPEM(data); err != nil {
			return nil, fmt.Errorf("parse public key %s: %w", publicKeyPath, err)
		}
	}

	return NewJWTManager(priv, pub), nil
}
