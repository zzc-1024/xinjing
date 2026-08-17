// Package persistence 提供持久化能力：数据库连接、迁移、仓储、对象存储与缓存。
// 生产使用 PostgreSQL 协议数据库（YugabyteDB 多写集群），开发使用进程内 SQLite。
package persistence

import (
	"fmt"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"xinjing/internal/logging"
)

// Open 打开指定驱动与连接串的数据库，并配置连接池。
// 返回的 gorm.DB 供后续仓储层使用。
// driver 取值：sqlite / postgres / ydb（后两者走 PG 协议）。
func Open(driver, dsn string, maxOpen, maxIdle int) (*gorm.DB, error) {
	var dialector gorm.Dialector
	switch driver {
	case "postgres", "postgresql", "pg", "ydb", "yugabyte", "yugabytedb":
		dialector = postgres.Open(dsn)
	case "sqlite", "sqlite3", "":
		dialector = sqlite.Open(dsn)
	default:
		return nil, fmt.Errorf("unsupported database driver %q (supported: sqlite / postgres / ydb)", driver)
	}

	// 把 GORM 内部的 SQL 日志接入统一日志模块：只记录慢查询与错误，避免刷屏
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.New(logging.PrintfLogger{Logger: logging.For("gorm")}, logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// 连接池：GORM 底层是 database/sql 的 *sql.DB
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get underlying sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db, nil
}
