package middleware

import (
	"log"
	"net/http"
	"time"
)

// responseWriter 包装原始 ResponseWriter，用于捕获状态码
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Logger 记录每个请求的方法、路径、状态码、耗时
func Logger() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// 包装 writer 以捕获响应状态码
			wrapped := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// 执行下一个 handler
			next.ServeHTTP(wrapped, r)

			// 记录日志
			log.Printf("[心境] %s %s | %d | %v | %s",
				r.Method,
				r.URL.Path,
				wrapped.statusCode,
				time.Since(start),
				r.RemoteAddr,
			)
		})
	}
}