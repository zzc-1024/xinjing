package middleware

import (
	"log"
	"net/http"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func Logger() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			wrapped := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(wrapped, r)

			// 从 Context 中提取 traceID
			traceID := GetTraceID(r.Context())

			log.Printf("[心境] [%s] %s %s | %d | %v | %s",
				traceID,
				r.Method,
				r.URL.Path,
				wrapped.statusCode,
				time.Since(start),
				r.RemoteAddr,
			)
		})
	}
}