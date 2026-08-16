package middleware

import (
	"net/http"

	"github.com/google/uuid"

	"xinjing/internal/logging"
)

// Trace 为每个请求分配/透传 X-Request-ID，并将其提交到日志模块
// （logging.WithTraceID）。该请求内后续通过 logging.FromContext 输出的
// 日志都会自动携带同一个 trace_id，从而与访问日志、业务日志关联起来。
func Trace() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			traceID := r.Header.Get("X-Request-ID")
			if traceID == "" {
				traceID = uuid.New().String()
			}
			w.Header().Set("X-Request-ID", traceID)

			ctx := logging.WithTraceID(r.Context(), traceID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
