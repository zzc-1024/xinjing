package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"

	"xinjing/internal/logging"
)

// Config 集中管理所有应用配置。
// 项目拆分为两个可独立部署的服务：cmd/auth（认证）与 cmd/server（网关），
// 它们共享本配置结构，但各取所需字段。
type Config struct {
	// 通用
	AppEnv    string // 运行环境: development / production
	LogLevel  string // 日志级别: debug / info / warn / error
	LogFormat string // 日志格式: text / json

	// 网关服务（cmd/server）：业务数据库
	ServerPort    string // 网关监听端口
	DBDriver      string // 业务数据库驱动: sqlite / postgres / ydb
	DBDSN         string // 业务数据库连接串
	DBMaxOpen     int    // 连接池最大连接数
	DBMaxIdle     int    // 连接池最大空闲连接数
	DBAutoMigrate bool   // 启动时是否自动执行数据库迁移

	// 认证服务（cmd/auth）：auth 数据库（与业务数据库分离）
	AuthServerPort string // 认证服务监听端口
	AuthDBDriver   string // auth 数据库驱动
	AuthDBDSN      string // auth 数据库连接串

	StorageBackend        string // 对象存储后端: local(开发) / s3(生产，指向 RustFS 等 S3 兼容服务)
	StorageLocalDir       string // local 后端的存储根目录
	StorageS3Endpoint     string // S3 端点（RustFS 地址，如 http://127.0.0.1:9000）
	StorageS3Region       string // S3 区域（RustFS 一般填 us-east-1 即可）
	StorageS3Bucket       string // S3 桶名
	StorageS3AccessKey    string // S3 访问密钥
	StorageS3SecretKey    string // S3 秘密密钥
	StorageS3UsePathStyle bool   // 是否使用 path-style 寻址（RustFS 需要 true）

	CacheBackend string // 缓存后端: memory(默认，单机)；valkey 留待阶段 2

	JWTPrivateKeyPath string // JWT 签名私钥文件路径（PEM，签发节点需要）
	JWTPublicKeyPath  string // JWT 验签公钥文件路径（PEM，验证节点需要）
	AccessTTL         string // access token（JWT）有效期字符串，如 "15m"
	RefreshTTL        string // refresh token 有效期字符串，如 "720h"（30 天）
}

// Load 从 .env 文件和环境变量中加载配置
// 优先级：系统环境变量 > .env 文件 > 默认值
func Load() *Config {
	// 尝试加载 .env 文件，文件不存在时不报错（生产环境通常通过容器注入环境变量）
	if err := godotenv.Load(); err != nil {
		logging.For("config").Debug("no .env file found, using system environment variables", "error", err)
	}

	cfg := &Config{
		AppEnv:        getEnv("XINJING_APP_ENV", "development"),
		LogLevel:      getEnv("XINJING_LOG_LEVEL", "info"),
		LogFormat:     getEnv("XINJING_LOG_FORMAT", "text"),
		ServerPort:    getEnv("XINJING_SERVER_PORT", "8080"),
		DBDriver:      getEnv("XINJING_DB_DRIVER", "sqlite"),
		DBDSN:         getEnv("XINJING_DB_DSN", "xinjing.db"),
		DBMaxOpen:     getEnvInt("XINJING_DB_MAX_OPEN", 10),
		DBMaxIdle:     getEnvInt("XINJING_DB_MAX_IDLE", 5),
		DBAutoMigrate: getEnvBool("XINJING_DB_AUTO_MIGRATE", true),

		AuthServerPort: getEnv("XINJING_AUTH_SERVER_PORT", "8081"),
		AuthDBDriver:   getEnv("XINJING_AUTH_DB_DRIVER", "sqlite"),
		AuthDBDSN:      getEnv("XINJING_AUTH_DB_DSN", "xinjing_auth.db"),

		StorageBackend:        getEnv("XINJING_STORAGE_BACKEND", "local"),
		StorageLocalDir:       getEnv("XINJING_STORAGE_LOCAL_DIR", "./storage"),
		StorageS3Endpoint:     getEnv("XINJING_STORAGE_S3_ENDPOINT", ""),
		StorageS3Region:       getEnv("XINJING_STORAGE_S3_REGION", "us-east-1"),
		StorageS3Bucket:       getEnv("XINJING_STORAGE_S3_BUCKET", ""),
		StorageS3AccessKey:    getEnv("XINJING_STORAGE_S3_ACCESS_KEY", ""),
		StorageS3SecretKey:    getEnv("XINJING_STORAGE_S3_SECRET_KEY", ""),
		StorageS3UsePathStyle: getEnvBool("XINJING_STORAGE_S3_USE_PATH_STYLE", true),

		CacheBackend: getEnv("XINJING_CACHE_BACKEND", "memory"),

		JWTPrivateKeyPath: getEnv("XINJING_JWT_PRIVATE_KEY", ""),
		JWTPublicKeyPath:  getEnv("XINJING_JWT_PUBLIC_KEY", ""),
		AccessTTL:         getEnv("XINJING_ACCESS_TTL", "15m"),
		RefreshTTL:        getEnv("XINJING_REFRESH_TTL", "720h"),
	}

	logging.For("config").Info("config loaded",
		"env", cfg.AppEnv,
		"port", cfg.ServerPort,
		"log_level", cfg.LogLevel,
		"log_format", cfg.LogFormat,
		"db_driver", cfg.DBDriver,
		"db_auto_migrate", cfg.DBAutoMigrate,
	)

	return cfg
}

// getEnv 读取环境变量，不存在时返回默认值
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// getEnvInt 读取整数环境变量，不存在或非法时返回默认值
func getEnvInt(key string, defaultValue int) int {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		logging.For("config").Warn("invalid int env, using default",
			"key", key, "value", value, "default", defaultValue)
		return defaultValue
	}
	return n
}

// getEnvBool 读取布尔环境变量，不存在或非法时返回默认值
func getEnvBool(key string, defaultValue bool) bool {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}
	b, err := strconv.ParseBool(value)
	if err != nil {
		logging.For("config").Warn("invalid bool env, using default",
			"key", key, "value", value, "default", defaultValue)
		return defaultValue
	}
	return b
}
