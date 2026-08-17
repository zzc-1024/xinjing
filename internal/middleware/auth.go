package middleware

import (
	"errors"
	"net/http"

	"xinjing/internal/auth"
	"xinjing/internal/logging"
	"xinjing/internal/response"
)

// Authenticate 用给定认证器校验请求身份，成功后将 Principal 注入请求上下文，
// 供后续 handler 通过 auth.FromContext 读取做授权与审计。
//
// 校验失败统一返回 401，但按「未带凭证」与「凭证无效」给出不同提示：
//   - ErrMissingCredentials → "missing credentials"
//   - 其它错误 → "invalid credentials"（不向客户端泄露具体失败原因，详情记入日志）
func Authenticate(authenticator auth.Authenticator) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, err := authenticator.Authenticate(r.Context(), r)
			if err != nil {
				logging.FromContext(r.Context()).Warn("authentication failed", "error", err)
				if errors.Is(err, auth.ErrMissingCredentials) {
					response.Error(w, http.StatusUnauthorized, "missing credentials")
				} else {
					response.Error(w, http.StatusUnauthorized, "invalid credentials")
				}
				return
			}
			ctx := auth.WithPrincipal(r.Context(), p)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireScope 检查已认证主体是否拥有指定 scope，无权限返回 403。
// 必须先经过 Authenticate：若上下文中没有 Principal（说明未认证），返回 401 而非 403。
// 区分 401/403：401=「我不知道你是谁」，403=「我知道你是谁，但你不许做这件事」。
func RequireScope(scope auth.Scope) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := auth.FromContext(r.Context())
			if !ok {
				response.Error(w, http.StatusUnauthorized, "authentication required")
				return
			}
			if !p.HasScope(scope) {
				response.Error(w, http.StatusForbidden, "insufficient scope")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
