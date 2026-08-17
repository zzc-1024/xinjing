package repo

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"xinjing/internal/persistence/models"
)

// APIKeyRepository 提供 API 密钥数据访问。
type APIKeyRepository interface {
	Repository[models.APIKey]
	// GetByKeyHash 按密钥哈希查询（鉴权时用）。
	GetByKeyHash(ctx context.Context, keyHash string) (*models.APIKey, error)
	// ListByUserID 列出某用户的全部密钥。
	ListByUserID(ctx context.Context, userID string) ([]models.APIKey, error)
}

type apiKeyRepository struct {
	Repository[models.APIKey]
	db *gorm.DB
}

// NewAPIKeyRepository 创建 API 密钥仓储。
func NewAPIKeyRepository(db *gorm.DB) APIKeyRepository {
	return &apiKeyRepository{
		Repository: NewRepository[models.APIKey](db),
		db:         db,
	}
}

func (r *apiKeyRepository) GetByKeyHash(ctx context.Context, keyHash string) (*models.APIKey, error) {
	var k models.APIKey
	if err := r.db.WithContext(ctx).Where("key_hash = ?", keyHash).First(&k).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &k, nil
}

func (r *apiKeyRepository) ListByUserID(ctx context.Context, userID string) ([]models.APIKey, error) {
	var keys []models.APIKey
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}
