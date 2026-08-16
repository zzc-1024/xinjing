package handler

import (
	"net/http"

	"xinjing/internal/logging"
	"xinjing/internal/response"
)

// Pinger 抽象「数据库连通性探测」。*sql.DB 天然满足此接口。
type Pinger interface {
	Ping() error
}

// HealthCheck 健康检查：报告服务与数据库状态。
// 数据库不可用时返回 503（服务仍存活，但不适合继续接流量）。
func HealthCheck(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(); err != nil {
			logging.FromContext(r.Context()).Error("db ping failed", "error", err)
			response.JSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "degraded",
				"db":     "down",
			})
			return
		}
		response.JSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"project": "xinjing",
			"db":      "up",
		})
	}
}
