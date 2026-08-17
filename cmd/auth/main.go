// cmd/auth 是心境平台的认证服务。
// 职责：维护独立的 auth 数据库（users、refresh_tokens），签发/校验 JWT，提供令牌端点。
// 与网关服务（cmd/server）分离部署：网关只持公钥验签，不接触 auth 数据库。
package main

import (
	"context"
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
	"xinjing/internal/persistence/migrate/authmigrations"
	"xinjing/internal/persistence/repo"
)

func main() {
	cfg := config.Load()

	logging.Init(logging.Config{
		Level:  logging.ParseLevel(cfg.LogLevel),
		Format: logging.ParseFormat(cfg.LogFormat),
	})
	log := logging.For("auth")

	// 1. 打开 auth 数据库（与业务数据库分离）
	db, err := persistence.Open(cfg.AuthDBDriver, cfg.AuthDBDSN, cfg.DBMaxOpen, cfg.DBMaxIdle)
	if err != nil {
		log.Error("open auth database failed", "error", err)
		os.Exit(1)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Error("get sql.DB failed", "error", err)
		os.Exit(1)
	}

	// 2. 运行 auth 迁移（users、refresh_tokens）
	if cfg.DBAutoMigrate {
		if err := migrate.Run(context.Background(), sqlDB, cfg.AuthDBDriver, authmigrations.FS); err != nil {
			log.Error("migrate auth database failed", "error", err)
			os.Exit(1)
		}
	}

	// 3. 认证服务是签发节点，私钥必须配置
	if cfg.JWTPrivateKeyPath == "" {
		log.Error("JWT private key not configured", "hint", "run go run ./cmd/keygen then set XINJING_JWT_PRIVATE_KEY")
		os.Exit(1)
	}
	jwtManager, err := auth.LoadJWTManager(cfg.JWTPrivateKeyPath, cfg.JWTPublicKeyPath)
	if err != nil {
		log.Error("load JWT keys failed", "error", err)
		os.Exit(1)
	}

	// 4. TTL 解析
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

	// 5. 令牌处理器（认证服务专有）
	tokenHandler := handler.NewTokenHandler(
		repo.NewUserRepository(db),
		repo.NewRefreshTokenRepository(db),
		jwtManager,
		accessTTL, refreshTTL,
	)

	// 6. 路由：认证服务只暴露令牌端点与健康检查，不需要 JWT 认证中间件
	//    （这些端点本身就是「获取/兑换/撤销令牌」的入口）。
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", handler.HealthCheck(sqlDB))
	mux.HandleFunc("POST /token", tokenHandler.HandleToken)
	mux.HandleFunc("POST /refresh", tokenHandler.HandleRefresh)
	mux.HandleFunc("POST /revoke", tokenHandler.HandleRevoke)

	finalHandler := middleware.Chain(
		mux,
		middleware.Recovery(),
		middleware.Trace(),
		middleware.AccessLog(),
		middleware.CORS(),
	)

	server := &http.Server{
		Addr:         ":" + cfg.AuthServerPort,
		Handler:      finalHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	go func() {
		log.Info("auth server starting", "port", cfg.AuthServerPort, "env", cfg.AppEnv)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("auth server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown error", "error", err)
	}
	log.Info("auth server stopped")
}
