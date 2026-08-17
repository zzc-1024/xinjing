package main

import (
	"context"
	"crypto/rsa"
	"fmt"
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

	// JWT 认证器（所有受保护路由统一用它校验 Bearer JWT）
	jwtAuthenticator := &auth.JWTAuthenticator{Manager: jwtManager}

	// 7. 路由注册
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", handler.HealthCheck(sqlDB))
	mux.HandleFunc("POST /token", tokenHandler.HandleToken)
	mux.HandleFunc("POST /refresh", tokenHandler.HandleRefresh)
	mux.HandleFunc("POST /revoke", tokenHandler.HandleRevoke)

	// 受保护路由示例：GET /me 需要「已认证 + 拥有 read 权限」。
	// Chain 从后往前包裹：先执行 Authenticate（认证）→ 再 RequireScope（授权）。
	mux.Handle("GET /me", middleware.Chain(
		http.HandlerFunc(handler.Me),
		middleware.Authenticate(jwtAuthenticator),
		middleware.RequireScope(auth.ScopeRead),
	))

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
//
// 本服务是「签发节点」：/token 和 /refresh 都要签发 JWT，因此私钥必不可少。
// 规则：
//   - 未配置私钥路径 → 立即报错退出（fail fast），而不是留到运行期让接口返回 500。
//   - 配置了路径但文件读不到或解析失败 → 报错退出。
//   - 公钥可选：配置了必须能解析；未配置则 Verify 能力缺失（纯签发节点不需要）。
func newJWTManager(cfg *config.Config) (*auth.JWTManager, error) {
	// 私钥必须配置
	if cfg.JWTPrivateKeyPath == "" {
		return nil, fmt.Errorf("JWT 私钥未配置：请用 go run ./cmd/keygen 生成密钥对，并设置 XINJING_JWT_PRIVATE_KEY")
	}
	privPEM, err := os.ReadFile(cfg.JWTPrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("读取 JWT 私钥文件 %s: %w", cfg.JWTPrivateKeyPath, err)
	}
	priv, err := auth.ParseRSAPrivateKeyPEM(privPEM)
	if err != nil {
		return nil, fmt.Errorf("解析 JWT 私钥文件 %s: %w", cfg.JWTPrivateKeyPath, err)
	}

	// 公钥可选，但若配置了必须能正确解析
	var pub *rsa.PublicKey
	if cfg.JWTPublicKeyPath != "" {
		pubPEM, err := os.ReadFile(cfg.JWTPublicKeyPath)
		if err != nil {
			return nil, fmt.Errorf("读取 JWT 公钥文件 %s: %w", cfg.JWTPublicKeyPath, err)
		}
		if pub, err = auth.ParseRSAPublicKeyPEM(pubPEM); err != nil {
			return nil, fmt.Errorf("解析 JWT 公钥文件 %s: %w", cfg.JWTPublicKeyPath, err)
		}
	}

	return auth.NewJWTManager(priv, pub), nil
}
