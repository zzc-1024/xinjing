package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"time"

	"xinjing/internal/config"
	"xinjing/internal/handler"
	"xinjing/internal/logging"
	"xinjing/internal/middleware"
	"xinjing/internal/persistence"
	"xinjing/internal/persistence/migrate"
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

	// 3. 打开数据库并配置连接池
	db, err := persistence.Open(cfg)
	if err != nil {
		log.Error("open database failed", "error", err)
		os.Exit(1)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Error("get sql.DB failed", "error", err)
		os.Exit(1)
	}

	// 4. 启动时自动执行数据库迁移（多实例安全）
	if cfg.DBAutoMigrate {
		if err := migrate.Run(context.Background(), sqlDB, cfg.DBDriver); err != nil {
			log.Error("migrate failed", "error", err)
			os.Exit(1)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", handler.HealthCheck(sqlDB))

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

	// 5. 监听操作系统中断信号（Ctrl+C）。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// 6. 在后台 goroutine 启动服务器，不阻塞主函数。
	go func() {
		log.Info("server starting", "port", cfg.ServerPort, "env", cfg.AppEnv)
		// 正常关闭时 ListenAndServe 返回 http.ErrServerClosed，这不是错误。
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// 7. 阻塞等待中断信号。
	<-ctx.Done()
	log.Info("shutting down...")

	// 8. 给服务器 10 秒完成现有请求，超时则强制关闭。
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown error", "error", err)
	}
	log.Info("server stopped")
}
