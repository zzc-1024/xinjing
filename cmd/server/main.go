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

	// 注册路由
	mux.HandleFunc("GET /ping", handler.HealthCheck)

	// 组装中间件链（从上到下依次执行）
	finalHandler := middleware.Chain(
		mux,
		middleware.Recovery(),  // 最外层：兜底 panic
		middleware.Logger(),    // 第二层：记录日志
		middleware.CORS(),      // 第三层：处理跨域
	)

	// 配置 HTTP Server
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