package main

import (
	"log"
	"net/http"
	"time"
	"xinjing/internal/config"
	"xinjing/internal/handler"
	"xinjing/internal/middleware"
)

func main() {
	// 加载配置
	cfg := config.Load()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", handler.HealthCheck)

	// 将中间件按顺序链式组合
	finalHandler := middleware.Chain(
		mux,
		middleware.Recovery(),
		middleware.Trace(),
		middleware.Logger(),
		middleware.CORS(),
	)

	server := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      finalHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("[心境] Server is running on :%s (env: %s)", cfg.ServerPort, cfg.AppEnv)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("[心境] Failed to start server: %v", err)
	}
}