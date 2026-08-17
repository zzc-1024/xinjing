package objectstore

import (
	"fmt"

	"xinjing/internal/config"
)

// New 根据配置构造对象存储实现：
// backend=local 用本机磁盘；backend=s3 连任何 S3 兼容服务（RustFS）。
func New(cfg *config.Config) (ObjectStore, error) {
	switch cfg.StorageBackend {
	case "local", "":
		return NewLocal(cfg.StorageLocalDir)
	case "s3":
		return NewS3(S3Config{
			Endpoint:     cfg.StorageS3Endpoint,
			Region:       cfg.StorageS3Region,
			Bucket:       cfg.StorageS3Bucket,
			AccessKey:    cfg.StorageS3AccessKey,
			SecretKey:    cfg.StorageS3SecretKey,
			UsePathStyle: cfg.StorageS3UsePathStyle,
		})
	default:
		return nil, fmt.Errorf("unsupported storage backend %q (支持: local / s3)", cfg.StorageBackend)
	}
}

// 保证编译期检查：Local 实现满足 ObjectStore 接口。
var _ ObjectStore = (*Local)(nil)
