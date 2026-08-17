package auth

import "context"

// AuthMethod 标识认证方式，供审计日志区分「这次请求是用什么凭证进来的」。
type AuthMethod string

const (
	// AuthMethodAPIKey 表示通过 API Key 认证。
	AuthMethodAPIKey AuthMethod = "apikey"
	// AuthMethodJWT 表示通过 JWT 认证。
	AuthMethodJWT AuthMethod = "jwt"
)

// Principal 描述认证成功后「谁在访问」的主体信息。
// 认证中间件校验通过后把它注入请求上下文，后续 handler 按需读取做授权与审计。
type Principal struct {
	UserID     string     // 用户 ID（UUIDv7 字符串）
	AuthMethod AuthMethod // 认证方式：apikey / jwt
	KeyID      string     // API Key 方式下为密钥记录 ID；JWT 方式下为空
	Scopes     []string   // 该凭证拥有的权限范围（原始字符串切片）
}

// HasScope 判断主体是否拥有指定权限（admin 通配全部，逻辑复用 Scopes.Has）。
func (p Principal) HasScope(scope Scope) bool {
	return NewScopes(p.Scopes).Has(scope)
}

// principalKey 是存放 Principal 的 context key。
// 用空结构体做 key 类型而非字符串，是为了避免不同包之间 key 命名冲突
// （字符串 key 可能被他人误用；自定义类型是包私有且唯一的）。
type principalKey struct{}

// WithPrincipal 把主体注入上下文，供请求内后续读取。
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// FromContext 从上下文读取主体。
// 第二个返回值 ok 表示是否存在：认证中间件未放行时 ctx 里没有主体，返回 ok=false。
func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}
