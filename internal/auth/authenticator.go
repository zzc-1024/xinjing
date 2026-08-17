package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

// ErrMissingCredentials 表示请求没有携带 Bearer JWT 凭证。
var ErrMissingCredentials = errors.New("missing credentials")

// Authenticator 是统一的认证接口：从 HTTP 请求中解析凭证并校验，成功返回主体。
// 当前平台只接受短期 JWT（Bearer token）；refresh token 不直接用于访问，
// 它在 /refresh 端点兑换成新的 JWT。故本接口目前只有一个 JWT 实现。
type Authenticator interface {
	Authenticate(ctx context.Context, r *http.Request) (Principal, error)
}

// JWTAuthenticator 用 JWT 认证（凭证来自 Authorization: Bearer 头）。
type JWTAuthenticator struct {
	Manager *JWTManager
}

// Authenticate 实现 Authenticator 接口。
func (a *JWTAuthenticator) Authenticate(ctx context.Context, r *http.Request) (Principal, error) {
	token := ExtractBearerToken(r)
	if token == "" {
		return Principal{}, ErrMissingCredentials
	}
	if a.Manager == nil {
		return Principal{}, ErrMissingKey
	}
	return a.Manager.Verify(ctx, token)
}

// ExtractBearerToken 从 Authorization: Bearer <token> 头提取 token。
// 只接受严格的 "Bearer xxx" 两段式写法，格式不对视为未携带。
func ExtractBearerToken(r *http.Request) string {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1]
	}
	return ""
}
