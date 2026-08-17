package migrate

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/glebarez/sqlite" // 注册纯 Go 的 "sqlite" 驱动（database/sql）
)

func TestRunSQLite(t *testing.T) {
	// 用临时目录里的真实文件库（纯内存库在多连接下会各自独立，不适合验证完整流程）
	dsn := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	// 第一次运行：应创建 users 表
	if err := Run(context.Background(), db, "sqlite"); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	// users 表应已存在
	var name string
	if err := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'users'`).Scan(&name); err != nil {
		t.Fatalf("users table not found: %v", err)
	}

	// goose 版本表应有 6 条实际迁移记录（version_id=0 是 goose 自动写入的基准哨兵行）
	// 对应 00001~00006 共 6 个迁移文件。
	var versions int
	if err := db.QueryRow(`SELECT count(*) FROM goose_db_version WHERE version_id > 0`).Scan(&versions); err != nil {
		t.Fatalf("query goose_db_version: %v", err)
	}
	if versions != 6 {
		t.Fatalf("goose_db_version count = %d, want 6", versions)
	}

	// 关键表都应已创建
	for _, table := range []string{"users", "api_keys", "functions", "function_versions", "routes", "rate_limit_policies", "plugins", "plugin_instances", "invocation_logs"} {
		var n int
		if err := db.QueryRow(
			`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&n); err != nil {
			t.Fatalf("query table %s: %v", table, err)
		}
		if n != 1 {
			t.Fatalf("table %s not created", table)
		}
	}

	// 迁移锁应已释放
	var locks int
	if err := db.QueryRow(`SELECT count(*) FROM schema_lock`).Scan(&locks); err != nil {
		t.Fatalf("query schema_lock: %v", err)
	}
	if locks != 0 {
		t.Fatalf("schema_lock rows = %d, want 0 (锁未释放)", locks)
	}

	// 第二次运行应幂等（不报错、不重复建表）
	if err := Run(context.Background(), db, "sqlite"); err != nil {
		t.Fatalf("second Run: %v", err)
	}
}

func TestRebind(t *testing.T) {
	in := `INSERT INTO t (a, b, c) VALUES (1, ?, ?) ON CONFLICT DO NOTHING`

	// SQLite：原样返回
	if got := rebind(in, false); got != in {
		t.Fatalf("rebind(sqlite) = %q, want unchanged %q", got, in)
	}
	// PostgreSQL：? 按出现顺序翻译为 $1、$2
	want := `INSERT INTO t (a, b, c) VALUES (1, $1, $2) ON CONFLICT DO NOTHING`
	if got := rebind(in, true); got != want {
		t.Fatalf("rebind(pg) = %q, want %q", got, want)
	}
}

func TestIsPostgresDriver(t *testing.T) {
	for _, d := range []string{"postgres", "postgresql", "pg", "ydb", "yugabyte", "yugabytedb"} {
		if !isPostgresDriver(d) {
			t.Errorf("isPostgresDriver(%q) = false, want true", d)
		}
	}
	for _, d := range []string{"sqlite", "sqlite3", "", "mysql"} {
		if isPostgresDriver(d) {
			t.Errorf("isPostgresDriver(%q) = true, want false", d)
		}
	}
}
