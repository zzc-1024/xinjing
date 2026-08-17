package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"xinjing/internal/persistence/models"
)

// refreshPrefix 是 refresh token 的固定前缀，肉眼识别 + 日志脱敏定位。
const refreshPrefix = "rt_"

// 校验 refresh token 可能返回的错误。
var (
	// ErrInvalidRefreshToken 表示 token 非法（格式错 / 哈希不匹配 / 记录不存在）。
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	// ErrRefreshTokenRevoked 表示 token 已被吊销。
	ErrRefreshTokenRevoked = errors.New("refresh token revoked")
	// ErrRefreshTokenExpired 表示 token 已过期。
	ErrRefreshTokenExpired = errors.New("refresh token expired")
)

// GenerateRefreshToken 生成一个明文 refresh token：rt_ + 32 字节随机数的 base64url 编码。
// 明文只在生成时返回一次，调用方应立刻持久化其哈希，绝不明文入库。
func GenerateRefreshToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return refreshPrefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

// HashRefreshToken 计算 refresh token 的 SHA-256 哈希（十六进制，64 字符）。
func HashRefreshToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// RefreshTokenStore 是 refresh token 查询的最小接口，供校验与兑换使用。
// repo.RefreshTokenRepository 天然满足该接口。
type RefreshTokenStore interface {
	GetByTokenHash(ctx context.Context, hash string) (*models.RefreshToken, error)
	Update(ctx context.Context, token *models.RefreshToken) error
}

// ValidateRefreshToken 校验明文 refresh token，成功返回对应的记录。
// 校验顺序：空值 → 查哈希（不存在/出错统一 ErrInvalidRefreshToken）→ 吊销 → 过期。
// 它只做「校验」，不负责兑换（rotation 在兑换环节由上层调用方完成）。
func ValidateRefreshToken(ctx context.Context, store RefreshTokenStore, plaintext string) (*models.RefreshToken, error) {
	if plaintext == "" {
		return nil, ErrInvalidRefreshToken
	}
	token, err := store.GetByTokenHash(ctx, HashRefreshToken(plaintext))
	if err != nil {
		// 无论「不存在」还是数据库故障，对外统一 ErrInvalidRefreshToken，
		// 避免泄露「这个哈希在不在库里」的信息。
		return nil, ErrInvalidRefreshToken
	}
	if token.RevokedAt != nil {
		return nil, ErrRefreshTokenRevoked
	}
	if time.Now().After(token.ExpiresAt) {
		return nil, ErrRefreshTokenExpired
	}
	return token, nil
}
