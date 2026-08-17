package repo

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"xinjing/internal/persistence/models"
)

// RefreshTokenRepository 提供 refresh token 的数据访问。
type RefreshTokenRepository interface {
	// Create 插入一条 refresh token 记录（主键由 UUIDv7 回调生成）。
	Create(ctx context.Context, token *models.RefreshToken) error
	// GetByTokenHash 按哈希查询（兑换时用）。
	GetByTokenHash(ctx context.Context, hash string) (*models.RefreshToken, error)
	// Update 全量保存（旋转刷新、滑动过期、吊销标记都走它）。
	Update(ctx context.Context, token *models.RefreshToken) error
	// ListByUserID 列出某用户的全部 refresh token（管理时用）。
	ListByUserID(ctx context.Context, userID string) ([]models.RefreshToken, error)
}

type refreshTokenRepository struct {
	db *gorm.DB
}

// NewRefreshTokenRepository 创建 refresh token 仓储。
func NewRefreshTokenRepository(db *gorm.DB) RefreshTokenRepository {
	return &refreshTokenRepository{db: db}
}

func (r *refreshTokenRepository) Create(ctx context.Context, token *models.RefreshToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *refreshTokenRepository) GetByTokenHash(ctx context.Context, hash string) (*models.RefreshToken, error) {
	var t models.RefreshToken
	if err := r.db.WithContext(ctx).Where("token_hash = ?", hash).First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (r *refreshTokenRepository) Update(ctx context.Context, token *models.RefreshToken) error {
	return r.db.WithContext(ctx).Save(token).Error
}

func (r *refreshTokenRepository) ListByUserID(ctx context.Context, userID string) ([]models.RefreshToken, error) {
	var tokens []models.RefreshToken
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&tokens).Error; err != nil {
		return nil, err
	}
	return tokens, nil
}

// 说明：过期/吊销判断不属于仓储职责，交由 auth 层完成；此处仅提供数据访问。
