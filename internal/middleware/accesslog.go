package middleware

import (
	"net/http"
	"time"

	"xinjing/internal/logging"
)

// responseWriter 记录响应状态码，供访问日志使用。
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// AccessLog 记录每个请求的访问信息（方法、路径、状态码、耗时、来源地址）。
// 通过 logging.FromContext 输出，若请求经过 Trace 中间件则自动携带 trace_id。
func AccessLog() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			logging.FromContext(r.Context()).Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.statusCode,
				"duration", time.Since(start),
				"remote", r.RemoteAddr,
			)
		})
	}
}
