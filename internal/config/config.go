package config

import (
	"os"

	"github.com/joho/godotenv"

	"xinjing/internal/logging"
)

// Config 集中管理所有应用配置
type Config struct {
	ServerPort string // HTTP 监听端口
	AppEnv     string // 运行环境: development / production
	LogLevel   string // 日志级别: debug / info / warn / error
	LogFormat  string // 日志格式: text / json
}

// Load 从 .env 文件和环境变量中加载配置
// 优先级：系统环境变量 > .env 文件 > 默认值
func Load() *Config {
	// 尝试加载 .env 文件，文件不存在时不报错（生产环境通常通过容器注入环境变量）
	if err := godotenv.Load(); err != nil {
		logging.For("config").Debug("no .env file found, using system environment variables", "error", err)
	}

	cfg := &Config{
		ServerPort: getEnv("XINJING_SERVER_PORT", "8080"),
		AppEnv:     getEnv("XINJING_APP_ENV", "development"),
		LogLevel:   getEnv("XINJING_LOG_LEVEL", "info"),
		LogFormat:  getEnv("XINJING_LOG_FORMAT", "text"),
	}

	logging.For("config").Info("config loaded",
		"env", cfg.AppEnv,
		"port", cfg.ServerPort,
		"log_level", cfg.LogLevel,
		"log_format", cfg.LogFormat,
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
