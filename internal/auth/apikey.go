package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"xinjing/internal/persistence/models"
)

// apiKeyPrefix 是平台签发的 API Key 固定前缀。
// 作用有二：肉眼即可识别这是本平台的密钥；日志脱敏时便于定位敏感字段。
const apiKeyPrefix = "xj_"

// API Key 校验过程可能返回的错误。调用方可用 errors.Is 精确区分。
var (
	// ErrInvalidKey 表示密钥格式非法或内容不匹配（含「记录不存在」）。
	ErrInvalidKey = errors.New("invalid api key")
	// ErrKeyRevoked 表示密钥已被吊销。
	ErrKeyRevoked = errors.New("api key revoked")
	// ErrKeyExpired 表示密钥已过期。
	ErrKeyExpired = errors.New("api key expired")
)

// GenerateAPIKey 生成一个新的明文 API Key，格式为 xj_ + 32 字节随机数的 base64url 编码。
// 32 字节（256 位）随机数足够抵御暴力猜测；base64url 只含 URL 安全字符，便于放在 Header 中。
// 明文只在本次返回一次，调用方应立刻把「哈希结果」持久化，绝不明文入库。
func GenerateAPIKey() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return apiKeyPrefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

// HashAPIKey 计算密钥的 SHA-256 哈希，返回十六进制字符串（64 字符）。
// 数据库只存哈希：即使数据泄露，攻击者也无法从哈希还原出明文密钥。
func HashAPIKey(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// APIKeyStore 是密钥查询的最小接口，供 AuthenticateKey 使用。
// repo.APIKeyRepository 天然满足该接口（它有 GetByKeyHash 方法），
// 从而让 auth 包与 GORM/仓储实现解耦，符合「业务只依赖接口」的约定。
type APIKeyStore interface {
	GetByKeyHash(ctx context.Context, keyHash string) (*models.APIKey, error)
}

// AuthenticateKey 校验明文密钥并返回访问者主体。校验顺序：
//  1. 空密钥 → ErrInvalidKey
//  2. 查哈希 → 查不到（或底层出错）统一返回 ErrInvalidKey，避免泄露「记录是否存在」
//  3. 状态非 active → ErrKeyRevoked
//  4. 已过 expires_at → ErrKeyExpired
func AuthenticateKey(ctx context.Context, store APIKeyStore, plaintext string) (Principal, error) {
	if plaintext == "" {
		return Principal{}, ErrInvalidKey
	}
	key, err := store.GetByKeyHash(ctx, HashAPIKey(plaintext))
	if err != nil {
		// 无论「不存在」还是数据库故障，对外一律 ErrInvalidKey。
		// 若区分两种错误，等于告诉攻击者「这个密钥哈希存在/不存在」。
		return Principal{}, ErrInvalidKey
	}
	if key.Status != models.APIKeyStatusActive {
		return Principal{}, ErrKeyRevoked
	}
	if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
		return Principal{}, ErrKeyExpired
	}
	return Principal{
		UserID:     key.UserID,
		AuthMethod: AuthMethodAPIKey,
		KeyID:      key.ID,
		Scopes:     key.Scopes,
	}, nil
}

// ExtractAPIKey 从 HTTP 请求中提取 API Key，支持两种方式：
//  1. Authorization: Bearer <key>（推荐，语义清晰，便于网关层统一处理）
//  2. X-API-Key: <key>（方便脚本、服务间调用）
//
// 两者同时存在时以 Authorization 优先。都未提供时返回空字符串。
func ExtractAPIKey(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		// strings.Fields 按空白切分，兼容 "Bearer xxx" 与 "xxx" 两种写法。
		parts := strings.Fields(h)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return parts[1]
		}
		if len(parts) == 1 {
			return parts[0]
		}
	}
	return r.Header.Get("X-API-Key")
}
