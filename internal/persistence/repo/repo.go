// Package repo 定义数据访问仓储：业务层只依赖接口，不直接接触 GORM。
package repo

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// ErrNotFound 表示记录不存在（统一映射 GORM 的 ErrRecordNotFound）。
var ErrNotFound = errors.New("record not found")

// Repository 是通用 CRUD 仓储接口。T 是实体类型，如 models.User。
type Repository[T any] interface {
	// Create 插入一条记录；主键为空时由模型钩子生成 UUIDv7。
	Create(ctx context.Context, entity *T) error
	// GetByID 按主键查询；不存在返回 ErrNotFound。
	GetByID(ctx context.Context, id string) (*T, error)
	// Update 全量保存实体。
	Update(ctx context.Context, entity *T) error
	// Delete 删除记录（有 DeletedAt 的模型走软删除）。
	Delete(ctx context.Context, id string) error
	// List 分页查询。
	List(ctx context.Context, offset, limit int) ([]T, error)
}

// gormRepository 是 Repository 的 GORM 通用实现。
type gormRepository[T any] struct {
	db *gorm.DB
}

// NewRepository 创建基于 GORM 的通用仓储。
func NewRepository[T any](db *gorm.DB) Repository[T] {
	return &gormRepository[T]{db: db}
}

func (r *gormRepository[T]) Create(ctx context.Context, entity *T) error {
	return r.db.WithContext(ctx).Create(entity).Error
}

func (r *gormRepository[T]) GetByID(ctx context.Context, id string) (*T, error) {
	var entity T
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&entity).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &entity, nil
}

func (r *gormRepository[T]) Update(ctx context.Context, entity *T) error {
	return r.db.WithContext(ctx).Save(entity).Error
}

func (r *gormRepository[T]) Delete(ctx context.Context, id string) error {
	var entity T
	return r.db.WithContext(ctx).Delete(&entity, "id = ?", id).Error
}

func (r *gormRepository[T]) List(ctx context.Context, offset, limit int) ([]T, error) {
	var entities []T
	if err := r.db.WithContext(ctx).Offset(offset).Limit(limit).Find(&entities).Error; err != nil {
		return nil, err
	}
	return entities, nil
}
