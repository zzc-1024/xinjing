package handler

import (
	"net/http"

	"xinjing/internal/response"
)

// HealthCheck 健康检查接口
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"project": "xinjing",
	})
}