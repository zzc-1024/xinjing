package objectstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// sumSuffix 是本地摘要侧车文件的扩展名（key+".sum" 存十六进制 sha256）。
const sumSuffix = ".sum"

// Local 把对象存到本机磁盘，用于开发与测试（零外部依赖）。
// key 直接映射为根目录下的相对路径；摘要存在同名 .sum 侧车文件中。
type Local struct {
	dir string
}

// NewLocal 创建 local 后端并确保根目录存在。
func NewLocal(dir string) (*Local, error) {
	if dir == "" {
		return nil, errors.New("local storage dir is empty")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create storage dir %q: %w", dir, err)
	}
	return &Local{dir: dir}, nil
}

// Put 把 r 的内容写入 key 对应文件，同时写 .sum 侧车文件。
func (l *Local) Put(ctx context.Context, key string, r io.Reader) (PutResult, error) {
	if err := validateKey(key); err != nil {
		return PutResult{}, err
	}
	path := l.path(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return PutResult{}, fmt.Errorf("mkdir for %q: %w", key, err)
	}

	hr := NewHashingReader(r)
	f, err := os.Create(path)
	if err != nil {
		return PutResult{}, fmt.Errorf("create %q: %w", key, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, hr); err != nil {
		return PutResult{}, fmt.Errorf("write %q: %w", key, err)
	}
	sum := hr.SumHex()
	if err := os.WriteFile(path+sumSuffix, []byte(sum), 0o644); err != nil {
		return PutResult{}, fmt.Errorf("write sum file for %q: %w", key, err)
	}
	return PutResult{Size: hr.Size(), Sum: sum}, nil
}

// Get 打开对象文件并返回；调用方负责 Close。
func (l *Local) Get(_ context.Context, key string) (io.ReadCloser, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}
	f, err := os.Open(l.path(key))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, key)
		}
		return nil, fmt.Errorf("open %q: %w", key, err)
	}
	return f, nil
}

// Delete 删除对象文件与 .sum 侧车文件（侧车不存在时忽略）。
func (l *Local) Delete(_ context.Context, key string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	path := l.path(key)
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: %s", ErrNotFound, key)
		}
		return fmt.Errorf("remove %q: %w", key, err)
	}
	_ = os.Remove(path + sumSuffix)
	return nil
}

// Stat 返回文件信息；摘要从 .sum 侧车读取（尽力而为，缺失则为空串）。
func (l *Local) Stat(_ context.Context, key string) (ObjectInfo, error) {
	if err := validateKey(key); err != nil {
		return ObjectInfo{}, err
	}
	info, err := os.Stat(l.path(key))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ObjectInfo{}, fmt.Errorf("%w: %s", ErrNotFound, key)
		}
		return ObjectInfo{}, fmt.Errorf("stat %q: %w", key, err)
	}
	sum := ""
	if b, err := os.ReadFile(l.path(key) + sumSuffix); err == nil {
		sum = string(b)
	}
	return ObjectInfo{Size: info.Size(), Sum: sum, ModTime: info.ModTime()}, nil
}

// Presign local 后端没有 HTTP 服务器，无法生成下载 URL，直接返回哨兵错误。
func (l *Local) Presign(key string, ttl time.Duration) (string, error) {
	return "", ErrPresignUnsupported
}

// path 把 key 映射为根目录下的绝对路径（key 已通过校验，不含危险成分）。
func (l *Local) path(key string) string {
	return filepath.Join(l.dir, filepath.FromSlash(key))
}
