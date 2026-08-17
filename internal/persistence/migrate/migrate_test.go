package migrate

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/glebarez/sqlite" // 注册纯 Go 的 "sqlite" 驱动（database/sql）

	"xinjing/internal/persistence/migrate/authmigrations"
	"xinjing/internal/persistence/migrate/gatewaymigrations"
)

// openSQLite 打开一个临时文件 SQLite 库。
func openSQLite(t *testing.T) *sql.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// assertTablesExist 断言给定表都已创建。
func assertTablesExist(t *testing.T, db *sql.DB, tables ...string) {
	t.Helper()
	for _, table := range tables {
		var n int
		if err := db.QueryRow(
			`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&n); err != nil {
			t.Fatalf("query table %s: %v", table, err)
		}
		if n != 1 {
			t.Fatalf("table %s not created", table)
		}
	}
}

// assertLockReleased 断言迁移锁已释放。
func assertLockReleased(t *testing.T, db *sql.DB) {
	t.Helper()
	var locks int
	if err := db.QueryRow(`SELECT count(*) FROM schema_lock`).Scan(&locks); err != nil {
		t.Fatalf("query schema_lock: %v", err)
	}
	if locks != 0 {
		t.Fatalf("schema_lock rows = %d, want 0 (lock not released)", locks)
	}
}

func TestRunAuthMigrations(t *testing.T) {
	db := openSQLite(t)

	if err := Run(context.Background(), db, "sqlite", authmigrations.FS); err != nil {
		t.Fatalf("first Run(auth): %v", err)
	}

	assertTablesExist(t, db, "users", "refresh_tokens")

	var colCount int
	if err := db.QueryRow(
		`SELECT count(*) FROM pragma_table_info('users') WHERE name = 'password_hash'`).Scan(&colCount); err != nil {
		t.Fatalf("query password_hash: %v", err)
	}
	if colCount != 1 {
		t.Fatalf("users.password_hash column not found")
	}

	var versions int
	if err := db.QueryRow(`SELECT count(*) FROM goose_db_version WHERE version_id > 0`).Scan(&versions); err != nil {
		t.Fatalf("query goose_db_version: %v", err)
	}
	if versions != 2 {
		t.Fatalf("auth goose_db_version count = %d, want 2", versions)
	}

	assertLockReleased(t, db)

	if err := Run(context.Background(), db, "sqlite", authmigrations.FS); err != nil {
		t.Fatalf("second Run(auth): %v", err)
	}
}

func TestRunGatewayMigrations(t *testing.T) {
	db := openSQLite(t)

	if err := Run(context.Background(), db, "sqlite", gatewaymigrations.FS); err != nil {
		t.Fatalf("first Run(gateway): %v", err)
	}

	assertTablesExist(t, db,
		"functions", "function_versions", "routes", "rate_limit_policies",
		"plugins", "plugin_instances", "invocation_logs",
	)

	var versions int
	if err := db.QueryRow(`SELECT count(*) FROM goose_db_version WHERE version_id > 0`).Scan(&versions); err != nil {
		t.Fatalf("query goose_db_version: %v", err)
	}
	if versions != 4 {
		t.Fatalf("gateway goose_db_version count = %d, want 4", versions)
	}

	assertLockReleased(t, db)
}

func TestRebind(t *testing.T) {
	in := `INSERT INTO t (a, b, c) VALUES (1, ?, ?) ON CONFLICT DO NOTHING`

	if got := rebind(in, false); got != in {
		t.Fatalf("rebind(sqlite) = %q, want unchanged %q", got, in)
	}
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
