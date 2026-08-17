package repo

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"xinjing/internal/persistence/models"
)

// RateLimitPolicyRepository 提供限流策略的数据访问。
// 复用通用 Repository[models.RateLimitPolicy] 的 CRUD，并扩展按名字查询
// （限流策略通常以 name 作为业务标识，路由据此引用）。
type RateLimitPolicyRepository interface {
	Repository[models.RateLimitPolicy]
	// GetByName 按策略名查询；不存在返回 ErrNotFound。
	GetByName(ctx context.Context, name string) (*models.RateLimitPolicy, error)
}

type rateLimitPolicyRepository struct {
	Repository[models.RateLimitPolicy]
	db *gorm.DB
}

// NewRateLimitPolicyRepository 创建限流策略仓储。
func NewRateLimitPolicyRepository(db *gorm.DB) RateLimitPolicyRepository {
	return &rateLimitPolicyRepository{
		Repository: NewRepository[models.RateLimitPolicy](db),
		db:         db,
	}
}

func (r *rateLimitPolicyRepository) GetByName(ctx context.Context, name string) (*models.RateLimitPolicy, error) {
	var p models.RateLimitPolicy
	if err := r.db.WithContext(ctx).Where("name = ?", name).First(&p).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}
