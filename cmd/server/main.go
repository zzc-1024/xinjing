package main

import (
	"context"
	"crypto/rsa"
	"net/http"
	"os"
	"os/signal"
	"time"

	"xinjing/internal/auth"
	"xinjing/internal/config"
	"xinjing/internal/handler"
	"xinjing/internal/logging"
	"xinjing/internal/middleware"
	"xinjing/internal/persistence"
	"xinjing/internal/persistence/migrate"
	"xinjing/internal/persistence/repo"
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

	// 5. 初始化 JWT 管理器（RSA 非对称签名）。
	// 签发节点需要私钥；验证节点需要公钥。二者可只配其一只用对应能力。
	jwtManager, err := newJWTManager(cfg)
	if err != nil {
		log.Error("init jwt manager failed", "error", err)
		os.Exit(1)
	}

	// 6. 初始化仓储与令牌处理器
	userRepo := repo.NewUserRepository(db)
	refreshRepo := repo.NewRefreshTokenRepository(db)

	accessTTL, err := time.ParseDuration(cfg.AccessTTL)
	if err != nil {
		log.Error("parse access TTL failed", "error", err)
		os.Exit(1)
	}
	refreshTTL, err := time.ParseDuration(cfg.RefreshTTL)
	if err != nil {
		log.Error("parse refresh TTL failed", "error", err)
		os.Exit(1)
	}

	tokenHandler := handler.NewTokenHandler(userRepo, refreshRepo, jwtManager, accessTTL, refreshTTL)

	// 7. 路由注册
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", handler.HealthCheck(sqlDB))
	mux.HandleFunc("POST /token", tokenHandler.HandleToken)
	mux.HandleFunc("POST /refresh", tokenHandler.HandleRefresh)

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

	// 8. 监听操作系统中断信号（Ctrl+C）。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// 9. 在后台 goroutine 启动服务器，不阻塞主函数。
	go func() {
		log.Info("server starting", "port", cfg.ServerPort, "env", cfg.AppEnv)
		// 正常关闭时 ListenAndServe 返回 http.ErrServerClosed，这不是错误。
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// 10. 阻塞等待中断信号。
	<-ctx.Done()
	log.Info("shutting down...")

	// 11. 给服务器 10 秒完成现有请求，超时则强制关闭。
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown error", "error", err)
	}
	log.Info("server stopped")
}

// newJWTManager 根据配置加载 RSA 密钥并构建 JWTManager。
// 私钥/公钥文件路径为空时对应能力缺失（Issue 或 Verify 返回 ErrMissingKey）。
// 签发节点配私钥，验证节点配公钥，二者可独立配置。
func newJWTManager(cfg *config.Config) (*auth.JWTManager, error) {
	var priv *rsa.PrivateKey
	var pub *rsa.PublicKey

	if cfg.JWTPrivateKeyPath != "" {
		pemData, err := os.ReadFile(cfg.JWTPrivateKeyPath)
		if err != nil {
			return nil, err
		}
		if priv, err = auth.ParseRSAPrivateKeyPEM(pemData); err != nil {
			return nil, err
		}
	}

	if cfg.JWTPublicKeyPath != "" {
		pemData, err := os.ReadFile(cfg.JWTPublicKeyPath)
		if err != nil {
			return nil, err
		}
		if pub, err = auth.ParseRSAPublicKeyPEM(pemData); err != nil {
			return nil, err
		}
	}

	return auth.NewJWTManager(priv, pub), nil
}
