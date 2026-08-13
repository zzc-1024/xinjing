package handler

import (
	"encoding/json"
	"net/http"
)

// HealthCheck 健康检查接口
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	resp := map[string]string{"status": "ok", "project": "xinjing"}
	json.NewEncoder(w).Encode(resp)
}