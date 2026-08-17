package objectstore

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"xinjing/internal/config"
)

// newTestLocal 创建指向临时目录的 local 后端。
func newTestLocal(t *testing.T) *Local {
	t.Helper()
	l, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}
	return l
}

func TestLocalRoundTrip(t *testing.T) {
	l := newTestLocal(t)
	ctx := context.Background()
	const key = "functions/abc/main.go"
	const content = "package main\n"

	// Put
	res, err := l.Put(ctx, key, strings.NewReader(content))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if res.Size != int64(len(content)) {
		t.Fatalf("Put size = %d, want %d", res.Size, len(content))
	}

	// Get 并读回内容
	rc, err := l.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	got, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != content {
		t.Fatalf("content = %q, want %q", got, content)
	}

	// Stat：大小与摘要应一致
	info, err := l.Stat(ctx, key)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size != int64(len(content)) {
		t.Fatalf("Stat size = %d, want %d", info.Size, len(content))
	}
	if info.Sum != res.Sum || info.Sum == "" {
		t.Fatalf("Stat sum = %q, want %q", info.Sum, res.Sum)
	}
	if info.ModTime.IsZero() {
		t.Fatal("Stat ModTime is zero")
	}

	// Delete 后再 Stat 应得到 ErrNotFound
	if err := l.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := l.Stat(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Stat after delete err = %v, want ErrNotFound", err)
	}
}

func TestLocalRejectsUnsafeKeys(t *testing.T) {
	l := newTestLocal(t)
	ctx := context.Background()
	for _, key := range []string{"../evil", "a/../../evil", "/abs", `a\b`} {
		if _, err := l.Put(ctx, key, strings.NewReader("x")); err == nil {
			t.Errorf("Put(%q) = nil error, want rejected", key)
		}
		if _, err := l.Get(ctx, key); err == nil {
			t.Errorf("Get(%q) = nil error, want rejected", key)
		}
	}
}

func TestLocalPresignUnsupported(t *testing.T) {
	l := newTestLocal(t)
	if _, err := l.Presign("a", 0); !errors.Is(err, ErrPresignUnsupported) {
		t.Fatalf("Presign err = %v, want ErrPresignUnsupported", err)
	}
}

func TestFactory(t *testing.T) {
	// local 后端
	l, err := New(&config.Config{StorageBackend: "local", StorageLocalDir: t.TempDir()})
	if err != nil {
		t.Fatalf("New(local): %v", err)
	}
	if _, ok := l.(*Local); !ok {
		t.Fatalf("New(local) type = %T, want *Local", l)
	}

	// 默认（空串）也走 local
	if _, err := New(&config.Config{StorageBackend: "", StorageLocalDir: t.TempDir()}); err != nil {
		t.Fatalf("New(default): %v", err)
	}

	// s3 缺 endpoint/bucket 应报错（不触发真实网络请求）
	if _, err := New(&config.Config{StorageBackend: "s3"}); err == nil {
		t.Fatal("New(s3, empty endpoint) = nil error, want error")
	}

	// 未知后端应报错
	if _, err := New(&config.Config{StorageBackend: "nfs"}); err == nil {
		t.Fatal("New(nfs) = nil error, want error")
	}
}
