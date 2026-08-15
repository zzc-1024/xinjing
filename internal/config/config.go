package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config 集中管理所有应用配置
type Config struct {
	ServerPort string // HTTP 监听端口
	AppEnv     string // 运行环境: development / production
	LogLevel   string // 日志级别: debug / info / warn / error
}

// Load 从 .env 文件和环境变量中加载配置
// 优先级：系统环境变量 > .env 文件 > 默认值
func Load() *Config {
	// 尝试加载 .env 文件，文件不存在时不报错（生产环境通常通过容器注入环境变量）
	if err := godotenv.Load(); err != nil {
		log.Println("[心境] No .env file found, using system environment variables")
	}

	cfg := &Config{
		ServerPort: getEnv("XINJING_SERVER_PORT", "8080"),
		AppEnv:     getEnv("XINJING_APP_ENV", "development"),
		LogLevel:   getEnv("XINJING_LOG_LEVEL", "info"),
	}

	log.Printf("[心境] Config loaded: env=%s port=%s log_level=%s",
		cfg.AppEnv, cfg.ServerPort, cfg.LogLevel)

	return cfg
}

// getEnv 读取环境变量，不存在时返回默认值
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}