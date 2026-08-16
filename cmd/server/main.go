package main

import (
	"net/http"
	"os"
	"time"

	"xinjing/internal/config"
	"xinjing/internal/handler"
	"xinjing/internal/logging"
	"xinjing/internal/middleware"
)

func main() {
	// 1. 加载配置（此阶段使用日志模块的引导默认 logger）
	cfg := config.Load()

	// 2. 根据配置初始化统一日志模块（等级、格式）
	logging.Init(logging.Config{
		Level:  logging.ParseLevel(cfg.LogLevel),
		Format: logging.ParseFormat(cfg.LogFormat),
	})

	log := logging.For("server")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", handler.HealthCheck)

	// Trace 作为请求身份边界，需位于访问日志外层，为请求注入 trace_id；
	// AccessLog 通过 logging.FromContext 读取，二者不再有代码级依赖。
	finalHandler := middleware.Chain(
		mux,
		middleware.Recovery(),
		middleware.Trace(),
		middleware.AccessLog(),
		middleware.CORS(),
	)

	server := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      finalHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Info("server starting", "port", cfg.ServerPort, "env", cfg.AppEnv)
	if err := server.ListenAndServe(); err != nil {
		log.Error("server failed to start", "error", err)
		os.Exit(1)
	}
}
