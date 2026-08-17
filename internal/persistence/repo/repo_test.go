package repo

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"xinjing/internal/persistence/models"
)

// openTestDB 用真实 SQLite 文件库 + GORM 建立测试环境。
// 这样能同时验证：模型映射、仓储实现、UUID 回调三者一致。
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/test.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// 注册全局 UUIDv7 主键回调
	if err := models.RegisterIDCallbacks(db); err != nil {
		t.Fatalf("register callbacks: %v", err)
	}
	// 用 GORM AutoMigrate 建表（表结构与迁移 SQL 对齐；迁移本身由 migrate 包测试覆盖）
	if err := db.AutoMigrate(
		&models.User{},
		&models.APIKey{},
		&models.Function{},
		&models.FunctionVersion{},
		&models.Route{},
		&models.RateLimitPolicy{},
		&models.Plugin{},
		&models.PluginInstance{},
		&models.InvocationLog{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	// 测试结束关闭连接，避免 SQLite 文件被占用导致 TempDir 清理失败
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestUserRepository(t *testing.T) {
	db := openTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	// Create：主键应被钩子生成 UUIDv7
	u := &models.User{Name: "张三", Email: "zhangsan@example.com"}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u.ID == "" {
		t.Fatal("Create 后 ID 为空，BeforeCreate 钩子未生效")
	}
	if len(u.ID) != 36 {
		t.Fatalf("ID 长度 = %d, want 36 (UUID)", len(u.ID))
	}

	// GetByID
	got, err := repo.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Email != "zhangsan@example.com" {
		t.Fatalf("email = %q, want zhangsan@example.com", got.Email)
	}

	// GetByEmail
	byEmail, err := repo.GetByEmail(ctx, "zhangsan@example.com")
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if byEmail.ID != u.ID {
		t.Fatalf("GetByEmail ID = %q, want %q", byEmail.ID, u.ID)
	}

	// Update
	u.Name = "李四"
	if err := repo.Update(ctx, u); err != nil {
		t.Fatalf("Update: %v", err)
	}
	updated, _ := repo.GetByID(ctx, u.ID)
	if updated.Name != "李四" {
		t.Fatalf("after update name = %q, want 李四", updated.Name)
	}

	// Delete（软删除）后再查应 ErrNotFound
	if err := repo.Delete(ctx, u.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, u.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByID after delete err = %v, want ErrNotFound", err)
	}
}

func TestAPIKeyRepository(t *testing.T) {
	db := openTestDB(t)
	repo := NewAPIKeyRepository(db)
	ctx := context.Background()

	key := &models.APIKey{
		UserID:  "user-1",
		KeyHash: "hashed-abc",
		Name:    "default",
		Scopes:  []string{"read", "write"},
		Status:  "active",
	}
	if err := repo.Create(ctx, key); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// GetByKeyHash
	got, err := repo.GetByKeyHash(ctx, "hashed-abc")
	if err != nil {
		t.Fatalf("GetByKeyHash: %v", err)
	}
	if got.UserID != "user-1" {
		t.Fatalf("user_id = %q, want user-1", got.UserID)
	}

	// ListByUserID
	list, err := repo.ListByUserID(ctx, "user-1")
	if err != nil {
		t.Fatalf("ListByUserID: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListByUserID len = %d, want 1", len(list))
	}

	// 不存在应 ErrNotFound
	if _, err := repo.GetByKeyHash(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByKeyHash(missing) err = %v, want ErrNotFound", err)
	}
}

func TestGenericRepository(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[models.Function](db)
	ctx := context.Background()

	fn := &models.Function{
		UserID:  "user-1",
		Name:    "hello",
		Runtime: "wasm",
		EnvVars: map[string]string{"REGION": "cn"},
	}
	if err := repo.Create(ctx, fn); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// GetByID 验证 serializer:json 的 map 字段正确读写
	got, err := repo.GetByID(ctx, fn.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.EnvVars["REGION"] != "cn" {
		t.Fatalf("EnvVars[REGION] = %q, want cn", got.EnvVars["REGION"])
	}

	// List
	list, err := repo.List(ctx, 0, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List len = %d, want 1", len(list))
	}

	// Delete 软删除后查询过滤
	if err := repo.Delete(ctx, fn.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, fn.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByID after delete err = %v, want ErrNotFound", err)
	}
}
