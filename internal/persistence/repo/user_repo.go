package repo

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"xinjing/internal/persistence/models"
)

// UserRepository 提供用户数据访问：复用通用 CRUD，并扩展按邮箱查询。
type UserRepository interface {
	Repository[models.User]
	GetByEmail(ctx context.Context, email string) (*models.User, error)
}

// userRepository 嵌入通用仓储以复用 CRUD，再补充特有方法。
type userRepository struct {
	Repository[models.User]
	db *gorm.DB
}

// NewUserRepository 创建用户仓储。
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{
		Repository: NewRepository[models.User](db),
		db:         db,
	}
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var u models.User
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}
