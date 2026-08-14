package main

import (
	"log"
	"net/http"
	"time"
	"xinjing/internal/handler"
	"xinjing/internal/middleware"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", handler.HealthCheck)
	
	// 组装中间件链
	finalHandler := middleware.Chain(
		mux,
		middleware.Recovery(), // 最外层兜底
		middleware.Trace(),    // 生成/提取 Trace ID 并注入 Context
		middleware.Logger(),   // 日志记录（此时 Context 中已有 Trace ID）
		middleware.CORS(),     // 处理跨域
	)

	server := &http.Server{
		Addr:         ":8080",
		Handler:      finalHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Println("[心境] Server is running on :8080")
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("[心境] Failed to start server: %v", err)
	}
}