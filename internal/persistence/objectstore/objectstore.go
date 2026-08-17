// Package objectstore 提供对象存储抽象：
// 业务代码只依赖 ObjectStore 接口，后端可无感替换——
// 开发用 local（本机磁盘），生产用 s3（任何 S3 兼容服务，如 RustFS）。
package objectstore

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"
)

// 本包对外暴露的哨兵错误，调用方可用 errors.Is 判断。
var (
	// ErrNotFound 表示对象不存在。
	ErrNotFound = errors.New("object not found")
	// ErrPresignUnsupported 表示该后端不支持预签名 URL。
	ErrPresignUnsupported = errors.New("presign is not supported by this backend")
)

// ObjectStore 是对象存储的抽象接口。
type ObjectStore interface {
	// Put 上传对象；key 为对象路径（如 "functions/abc/main.go"）。
	Put(ctx context.Context, key string, r io.Reader) (PutResult, error)
	// Get 下载对象；调用方负责关闭返回的 reader。
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	// Delete 删除对象；对象不存在时返回 ErrNotFound。
	Delete(ctx context.Context, key string) error
	// Stat 返回对象元信息；对象不存在时返回 ErrNotFound。
	Stat(ctx context.Context, key string) (ObjectInfo, error)
	// Presign 生成限时下载 URL；不支持的后端返回 ErrPresignUnsupported。
	Presign(key string, ttl time.Duration) (string, error)
}

// PutResult 是上传结果的元信息。
type PutResult struct {
	Size int64  // 对象字节数
	Sum  string // sha256 十六进制摘要
}

// ObjectInfo 是对象的元信息。
type ObjectInfo struct {
	Size    int64     // 对象字节数
	Sum     string    // sha256 十六进制摘要；后端不提供时为 ""
	ModTime time.Time // 最后修改时间；零值表示未知
}

// validateKey 校验对象 key 的合法性，防止路径穿越。
// S3 风格的 key 使用 "/" 分隔，因此反斜杠、"." 与 ".." 段、绝对路径一律拒绝。
func validateKey(key string) error {
	if key == "" {
		return errors.New("object key is empty")
	}
	if strings.ContainsAny(key, `\`) {
		return errors.New("object key must use '/' as separator")
	}
	if strings.HasPrefix(key, "/") {
		return errors.New("object key must be relative")
	}
	if strings.Contains(key, "..") {
		return errors.New("object key must not contain '..'")
	}
	return nil
}
