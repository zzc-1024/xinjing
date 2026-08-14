package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// traceIDKey 是用于在 context 中存储 traceID 的自定义类型
type traceIDKey struct{}

// GetTraceID 从 context 中提取 traceID（供后续业务 handler 或日志使用）
func GetTraceID(ctx context.Context) string {
	if id, ok := ctx.Value(traceIDKey{}).(string); ok {
		return id
	}
	return ""
}

// Trace 生成或提取请求追踪 ID
func Trace() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 优先从请求头中提取上游传来的 Trace ID
			traceID := r.Header.Get("X-Request-ID")
			
			// 如果上游没有传递，则生成一个新的 UUID
			if traceID == "" {
				traceID = uuid.New().String()
			}

			// 将 traceID 注入到请求的 Context 中，传递给下游
			ctx := context.WithValue(r.Context(), traceIDKey{}, traceID)

			// 将 traceID 写入响应头，方便调用方/前端排查问题
			w.Header().Set("X-Request-ID", traceID)

			// 使用包含新 Context 的请求继续执行
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}