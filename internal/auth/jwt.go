package auth

import (
	"context"
	"crypto/rsa"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWT 校验可能返回的错误。
var (
	// ErrInvalidToken 表示 JWT 非法（签名错误、格式错误、被篡改等）。
	ErrInvalidToken = errors.New("invalid token")
	// ErrTokenExpired 表示 JWT 已过期。
	ErrTokenExpired = errors.New("token expired")
	// ErrMissingKey 表示缺少签发/验证所需的密钥。
	ErrMissingKey = errors.New("missing key")
)

// JWTManager 负责 JWT 的签发与校验，采用 RSA 非对称签名（RS256）。
//
// 为什么用非对称而不是对称（HS256）？
// 对称方案全平台只有一把共享密钥，谁拿到都能「既签发又验证」；分布式部署时
// 每个节点、甚至要接入的第三方都得持有它，任何一点泄露就全盘失守。
// 非对称方案里：私钥只留在签发节点（能签不能给），公钥可放心分发给所有
// 验证节点与第三方（只能验证、无法伪造）。这正契合分布式、可对外的服务形态。
type JWTManager struct {
	privateKey *rsa.PrivateKey  // 签发用私钥；纯验证节点可传 nil
	publicKey  *rsa.PublicKey   // 验证用公钥；纯签发节点可传 nil
	now        func() time.Time // 可注入的时钟，测试时替换以模拟时间流逝
}

// NewJWTManager 用给定密钥对创建 JWTManager。
// 允许只传一个：签发节点只传 privateKey；验证节点/第三方只传 publicKey。
// 传 nil 的那一侧，对应的 Issue/Verify 会返回 ErrMissingKey。
func NewJWTManager(privateKey *rsa.PrivateKey, publicKey *rsa.PublicKey) *JWTManager {
	return &JWTManager{privateKey: privateKey, publicKey: publicKey, now: time.Now}
}

// jwtClaims 是自定义的 JWT 载荷，嵌入标准 RegisteredClaims 并追加 scopes。
type jwtClaims struct {
	Scopes []string `json:"scopes"`
	jwt.RegisteredClaims
}

// Issue 为用户签发一个 JWT，有效期 ttl。需要持有私钥（签发节点）。
func (m *JWTManager) Issue(ctx context.Context, userID string, scopes []string, ttl time.Duration) (string, error) {
	if m.privateKey == nil {
		return "", ErrMissingKey
	}
	now := m.now()
	claims := jwtClaims{
		Scopes: scopes,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Issuer:    "xinjing",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	// RS256 要求传入 RSA 私钥；SignedString 内部会用私钥对 header+payload 签名。
	return token.SignedString(m.privateKey)
}

// Verify 校验并解析 JWT，成功返回主体。需要持有公钥（验证节点/第三方）。
// 用 WithValidMethods 强制只接受 RS256，从根上杜绝「alg 混淆攻击」
// （攻击者把算法字段改成 none/HS256 等弱算法来绕过签名校验）。
func (m *JWTManager) Verify(ctx context.Context, tokenString string) (Principal, error) {
	if m.publicKey == nil {
		return Principal{}, ErrMissingKey
	}
	var claims jwtClaims
	token, err := jwt.ParseWithClaims(
		tokenString,
		&claims,
		func(t *jwt.Token) (any, error) { return m.publicKey, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
	)
	if err != nil {
		// 用 errors.Is 区分「过期」与其他错误，调用方可据此给出更精确的提示。
		if errors.Is(err, jwt.ErrTokenExpired) {
			return Principal{}, ErrTokenExpired
		}
		return Principal{}, ErrInvalidToken
	}
	if !token.Valid {
		return Principal{}, ErrInvalidToken
	}
	// Subject 是我们签发的 userID；为空说明是非法 token。
	if claims.Subject == "" {
		return Principal{}, ErrInvalidToken
	}
	return Principal{
		UserID:     claims.Subject,
		AuthMethod: AuthMethodJWT,
		Scopes:     claims.Scopes,
	}, nil
}
