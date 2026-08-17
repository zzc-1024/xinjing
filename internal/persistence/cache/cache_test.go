package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"xinjing/internal/config"
)

func TestMemoryRoundTrip(t *testing.T) {
	c := NewMemory()
	ctx := context.Background()

	// 初始未命中
	if _, err := c.Get(ctx, "k"); !errors.Is(err, ErrMiss) {
		t.Fatalf("Get(missing) err = %v, want ErrMiss", err)
	}

	// Set 后 Get
	val := []byte("hello")
	if err := c.Set(ctx, "k", val, time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := c.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("value = %q, want %q", got, "hello")
	}

	// Delete 后再 Get 应 Miss
	if err := c.Delete(ctx, "k"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := c.Get(ctx, "k"); !errors.Is(err, ErrMiss) {
		t.Fatalf("Get(after delete) err = %v, want ErrMiss", err)
	}

	// Delete 不存在的 key 不报错（幂等）
	if err := c.Delete(ctx, "nonexistent"); err != nil {
		t.Fatalf("Delete(nonexistent): %v", err)
	}
}

func TestMemoryExpiry(t *testing.T) {
	c := NewMemory()
	ctx := context.Background()

	if err := c.Set(ctx, "k", []byte("v"), 20*time.Millisecond); err != nil {
		t.Fatalf("Set: %v", err)
	}
	// 过期后应 Miss
	time.Sleep(30 * time.Millisecond)
	if _, err := c.Get(ctx, "k"); !errors.Is(err, ErrMiss) {
		t.Fatalf("Get(expired) err = %v, want ErrMiss", err)
	}

	// ttl<=0 表示永不过期
	if err := c.Set(ctx, "forever", []byte("v"), 0); err != nil {
		t.Fatalf("Set(forever): %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, err := c.Get(ctx, "forever"); err != nil {
		t.Fatalf("Get(forever) err = %v, want nil", err)
	}
}

func TestMemoryConcurrent(t *testing.T) {
	c := NewMemory()
	ctx := context.Background()

	// 每个 goroutine 用独立的 key，检验「Memory 在并发读写上的线程安全」。
	// 注意：不能所有 goroutine 共享同一个 key 再各自 Delete —— 那会让别的
	// goroutine 的 Delete 删掉此 goroutine 刚写入的值，导致 Get 读到 ErrMiss，
	// 这属于测试对并发交互的错误假设，而非 Memory 实现的缺陷。
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("k-%d", n)
			_ = c.Set(ctx, key, []byte("v"), time.Minute)
			if _, err := c.Get(ctx, key); err != nil {
				t.Errorf("concurrent Get: %v", err)
			}
			_ = c.Delete(ctx, key)
		}(i)
	}
	wg.Wait()
}

func TestMemoryIncrUnsupported(t *testing.T) {
	c := NewMemory()
	if _, err := c.Incr(context.Background(), "k"); err == nil {
		t.Fatal("Incr() = nil error, want unsupported error")
	}
}

func TestFactory(t *testing.T) {
	// memory 后端
	c, err := New(&config.Config{CacheBackend: "memory"})
	if err != nil {
		t.Fatalf("New(memory): %v", err)
	}
	if _, ok := c.(*Memory); !ok {
		t.Fatalf("New(memory) type = %T, want *Memory", c)
	}

	// 默认（空串）也走 memory
	if _, err := New(&config.Config{CacheBackend: ""}); err != nil {
		t.Fatalf("New(default): %v", err)
	}

	// 未知后端应报错
	if _, err := New(&config.Config{CacheBackend: "redis"}); err == nil {
		t.Fatal("New(redis) = nil error, want error")
	}
}
