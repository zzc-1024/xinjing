// Package logging 提供统一日志能力：日志等级、日志来源（组件）、输出格式，
// 以及请求级 trace 关联。全项目统一通过本包输出日志，不再各自拼接前缀。
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// Format 表示日志输出格式。
type Format uint8

const (
	FormatText Format = iota // 面向开发者的文本格式
	FormatJSON               // 面向生产的结构化格式
)

// Config 是日志模块的初始化配置。
type Config struct {
	Level  slog.Level // 全局最低输出等级
	Format Format     // 输出格式
}

// init 在读取配置之前先提供一个引导默认 logger（文本、Debug、stderr），
// 保证启动阶段（如加载 .env、读取配置）也能正常输出日志。
func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
}

// Init 根据配置重建全局默认 logger（应在读取配置后调用一次）。
func Init(cfg Config) {
	var h slog.Handler
	opts := &slog.HandlerOptions{Level: cfg.Level}
	switch cfg.Format {
	case FormatJSON:
		h = slog.NewJSONHandler(os.Stderr, opts)
	default:
		h = slog.NewTextHandler(os.Stderr, opts)
	}
	slog.SetDefault(slog.New(h))
}

// Default 返回当前全局默认 logger。
func Default() *slog.Logger {
	return slog.Default()
}

// For 返回绑定了指定日志来源（组件名）的 logger。
// 建议在调用点使用，以便始终反映最新的等级/格式配置。
func For(source string) *slog.Logger {
	return Default().With("source", source)
}

// ParseLevel 将字符串解析为日志等级，非法值回退到 Info。
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		if s != "" {
			For("logging").Warn("unknown log level, fallback to info", "value", s)
		}
		return slog.LevelInfo
	}
}

// ParseFormat 将字符串解析为日志格式，非法值回退到文本格式。
func ParseFormat(s string) Format {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "json":
		return FormatJSON
	case "text":
		return FormatText
	default:
		if s != "" {
			For("logging").Warn("unknown log format, fallback to text", "value", s)
		}
		return FormatText
	}
}
