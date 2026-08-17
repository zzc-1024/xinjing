package repo

import (
	"context"
	"errors"
	"testing"
	"time"

	"xinjing/internal/persistence/models"
)

func TestRefreshTokenRepository(t *testing.T) {
	db := openTestDB(t)
	repo := NewRefreshTokenRepository(db)
	ctx := context.Background()

	exp := time.Now().Add(24 * time.Hour)
	token := &models.RefreshToken{
		UserID:    "user-1",
		TokenHash: "hash-abc",
		GrantedTo: models.GrantedToSelf,
		Scopes:    []string{"read"},
		ExpiresAt: exp,
	}
	if err := repo.Create(ctx, token); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if token.ID == "" {
		t.Fatal("Create 后 ID 为空，UUID 回调未生效")
	}

	// GetByTokenHash
	got, err := repo.GetByTokenHash(ctx, "hash-abc")
	if err != nil {
		t.Fatalf("GetByTokenHash: %v", err)
	}
	if got.UserID != "user-1" {
		t.Fatalf("user_id = %q, want user-1", got.UserID)
	}
	if got.GrantedTo != models.GrantedToSelf {
		t.Fatalf("granted_to = %q, want self", got.GrantedTo)
	}
	if len(got.Scopes) != 1 || got.Scopes[0] != "read" {
		t.Fatalf("scopes = %v, want [read]", got.Scopes)
	}

	// Update（模拟旋转刷新 + 吊销）
	now := time.Now()
	got.RevokedAt = &now
	got.RotatedFrom = got.ID
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	updated, _ := repo.GetByTokenHash(ctx, "hash-abc")
	if updated.RevokedAt == nil {
		t.Fatal("RevokedAt 应为非空")
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
	if _, err := repo.GetByTokenHash(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByTokenHash(missing) err = %v, want ErrNotFound", err)
	}
}
