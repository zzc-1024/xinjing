package repo

import (
	"context"
	"errors"
	"testing"

	"xinjing/internal/persistence/models"
)

func TestRateLimitPolicyRepository(t *testing.T) {
	db := openTestDB(t)
	repo := NewRateLimitPolicyRepository(db)
	ctx := context.Background()

	p := &models.RateLimitPolicy{
		Name:       "me-default",
		LimitCount: 100,
		WindowSec:  60,
		Burst:      10,
		Scope:      "per-key",
	}
	if err := repo.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.ID == "" {
		t.Fatal("Create 后 ID 为空")
	}

	// GetByName
	got, err := repo.GetByName(ctx, "me-default")
	if err != nil {
		t.Fatalf("GetByName: %v", err)
	}
	if got.LimitCount != 100 {
		t.Fatalf("LimitCount = %d, want 100", got.LimitCount)
	}
	if got.Scope != "per-key" {
		t.Fatalf("Scope = %q, want per-key", got.Scope)
	}

	// 不存在应 ErrNotFound
	if _, err := repo.GetByName(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByName(missing) err = %v, want ErrNotFound", err)
	}
}
