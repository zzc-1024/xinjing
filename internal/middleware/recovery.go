package middleware

import (
	"net/http"
	"runtime/debug"

	"xinjing/internal/logging"
)

// Recovery 捕获 handler 中的 panic，防止整个服务崩溃。
func Recovery() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logging.FromContext(r.Context()).Error("panic recovered",
						"panic", err,
						"stack", string(debug.Stack()),
					)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
