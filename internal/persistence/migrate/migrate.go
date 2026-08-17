// Package migrate 管理数据库结构迁移：
// SQL 迁移文件经 go:embed 内嵌进二进制，启动时自动执行；
// 多实例并发启动时用表级锁（schema_lock）串行化——不使用 PG advisory lock，
// 以便同时兼容 SQLite 与 YugabyteDB。
package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pressly/goose/v3"

	"xinjing/internal/logging"
)

// lockTTL 是迁移锁的「租约」时长：持有者超过该时长未释放，视为崩溃，允许接管。
const lockTTL = 10 * time.Minute

// Run 应用 fsys 中的所有待执行迁移（幂等：无新迁移时直接返回）。
// driver 取值：sqlite / postgres / ydb。
// fsys 是调用方传入的迁移集合（auth 服务传 authmigrations.FS，网关传 gatewaymigrations.FS），
// 从而让不同服务各自维护独立的数据库 schema。
func Run(ctx context.Context, db *sql.DB, driver string, fsys fs.FS) error {
	dialect, err := dialectFor(driver)
	if err != nil {
		return err
	}

	// 先抢锁，防止多实例同时执行迁移互相冲突
	pg := isPostgresDriver(driver)
	token := uuid.New().String()
	if err := acquireLock(ctx, db, token, pg); err != nil {
		return fmt.Errorf("acquire schema lock: %w", err)
	}
	defer func() {
		if err := releaseLock(db, token, pg); err != nil {
			logging.For("migrate").Error("release schema lock", "error", err)
		}
	}()

	// goose 的日志接入统一日志模块
	goose.SetLogger(logging.PrintfLogger{Logger: logging.For("goose")})

	provider, err := goose.NewProvider(dialect, db, fsys)
	if err != nil {
		return fmt.Errorf("create goose provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

// acquireLock 通过 schema_lock 表抢锁：带重试、超时与崩溃接管。
// 所有 SQL 只使用 SQLite / PostgreSQL / YugabyteDB 共同支持的语法；
// 占位符按驱动翻译（SQLite 用 ?，PostgreSQL 系用 $1）。
func acquireLock(ctx context.Context, db *sql.DB, token string, pg bool) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_lock (
		id          INTEGER PRIMARY KEY,
		token       TEXT NOT NULL,
		acquired_at INTEGER NOT NULL
	)`); err != nil {
		return err
	}

	deadline := time.Now().Add(lockTTL)
	for {
		now := time.Now().Unix()

		// 尝试插入唯一锁行；已存在时 ON CONFLICT DO NOTHING 使影响行数为 0
		res, err := db.ExecContext(ctx,
			rebind(`INSERT INTO schema_lock (id, token, acquired_at) VALUES (1, ?, ?) ON CONFLICT (id) DO NOTHING`, pg),
			token, now)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 1 {
			return nil // 抢锁成功
		}

		// 锁在别人手上：若对方超过 TTL 未续租（可能已崩溃），直接接管
		cutoff := now - int64(lockTTL.Seconds())
		if _, err := db.ExecContext(ctx,
			rebind(`UPDATE schema_lock SET token = ?, acquired_at = ? WHERE id = 1 AND acquired_at < ?`, pg),
			token, now, cutoff); err != nil {
			return err
		}

		// 确认锁是否已归我们（可能刚接管成功，也可能被别的实例抢先）
		var holder string
		if err := db.QueryRowContext(ctx,
			`SELECT token FROM schema_lock WHERE id = 1`).Scan(&holder); err == nil && holder == token {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			return errors.New("schema lock held too long by another instance")
		}
	}
}

// releaseLock 只释放属于自己的锁（token 匹配才删），防止误删他人的锁。
func releaseLock(db *sql.DB, token string, pg bool) error {
	_, err := db.Exec(rebind(`DELETE FROM schema_lock WHERE id = 1 AND token = ?`, pg), token)
	return err
}

// isPostgresDriver 判断驱动是否使用 PostgreSQL 风格的 $1 占位符。
func isPostgresDriver(driver string) bool {
	switch driver {
	case "postgres", "postgresql", "pg", "ydb", "yugabyte", "yugabytedb":
		return true
	default:
		return false
	}
}

// rebind 把 ? 占位符翻译为 PostgreSQL 的 $1、$2、... 风格（SQLite 原样返回）。
// 注意：本实现只适用于「字符串字面量中不含 ?」的受控 SQL——本包内的锁 SQL 满足该前提。
func rebind(query string, pg bool) string {
	if !pg {
		return query
	}
	var sb strings.Builder
	sb.Grow(len(query) + 8)
	n := 0
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			n++
			sb.WriteByte('$')
			sb.WriteString(strconv.Itoa(n))
		} else {
			sb.WriteByte(query[i])
		}
	}
	return sb.String()
}

// dialectFor 把驱动名映射为 goose 方言。
func dialectFor(driver string) (goose.Dialect, error) {
	switch driver {
	case "sqlite", "sqlite3", "":
		return goose.DialectSQLite3, nil
	case "postgres", "postgresql", "pg":
		return goose.DialectPostgres, nil
	case "ydb", "yugabyte", "yugabytedb":
		return goose.DialectYdB, nil
	default:
		return goose.DialectCustom, fmt.Errorf("unsupported database driver %q (supported: sqlite / postgres / ydb)", driver)
	}
}
