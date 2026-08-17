package middleware

import (
	"net"
	"net/http"

	"xinjing/internal/auth"
	"xinjing/internal/logging"
	"xinjing/internal/ratelimit"
	"xinjing/internal/response"
)

// RateLimit 是限流中间件：按策略的 scope 维度从请求提取 key，
// 调用限流器判定是否放行；超限返回 429 Too Many Requests。
//
// 故障策略（fail-open）：限流器自身故障（如缓存不可用）时放行并记日志，
// 宁可「少保护」也不误杀正常请求；限流是保护手段，不是核心链路。
//
// 用法：middleware.Chain(handler, middleware.Authenticate(...), middleware.RateLimit(limiter, policy))
// 注意 Chain 从后往前包裹，RateLimit 应写在 Authenticate 之后（先认证，才能按用户限流）。
func RateLimit(limiter ratelimit.RateLimiter, policy ratelimit.Policy) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := scopeKey(r, policy.Scope)
			allowed, err := limiter.Allow(r.Context(), key, policy)
			if err != nil {
				// fail-open：限流器故障不拦请求，但记录日志供排查
				logging.FromContext(r.Context()).Warn("rate limiter error, fail open", "error", err, "key", key)
				next.ServeHTTP(w, r)
				return
			}
			if !allowed {
				logging.FromContext(r.Context()).Warn("rate limit exceeded", "key", key, "policy", policy.Name)
				response.Error(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// scopeKey 按限流维度提取标识：
//   - per-key：已认证用用户 ID，未认证退回客户端 IP
//   - per-route：用请求路径
//   - global：固定 "global"（全局共享一个配额）
func scopeKey(r *http.Request, scope ratelimit.Scope) string {
	switch scope {
	case ratelimit.ScopePerKey:
		if p, ok := auth.FromContext(r.Context()); ok && p.UserID != "" {
			return p.UserID
		}
		return clientIP(r)
	case ratelimit.ScopePerRoute:
		return r.URL.Path
	default:
		return "global"
	}
}

// clientIP 从 RemoteAddr 提取纯 IP（去掉端口）。解析失败时原样返回。
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
