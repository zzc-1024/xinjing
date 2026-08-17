package objectstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

// S3Config 是 S3 后端的连接参数（对应 config 中的 XINJING_STORAGE_S3_* 项）。
type S3Config struct {
	Endpoint     string // 服务端点，如 http://127.0.0.1:9000（RustFS）
	Region       string // 区域；RustFS 一般填 us-east-1
	Bucket       string // 桶名
	AccessKey    string // 访问密钥
	SecretKey    string // 秘密密钥
	UsePathStyle bool   // path-style 寻址（RustFS 需要 true）
}

// S3 使用 aws-sdk-go-v2 连接任何 S3 兼容服务（RustFS / SeaweedFS / 云 S3）。
type S3 struct {
	client  *s3.Client
	presign *s3.PresignClient
	bucket  string
}

// NewS3 创建 S3 后端；连接与密钥校验延迟到首次请求时才真正发生。
func NewS3(cfg S3Config) (*S3, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" {
		return nil, errors.New("s3 endpoint and bucket are required")
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = cfg.UsePathStyle
	})
	return &S3{client: client, presign: s3.NewPresignClient(client), bucket: cfg.Bucket}, nil
}

// Put 上传对象并附带 sha256 校验和，让服务端（RustFS/S3）验签。
// 当前实现把对象读入内存以计算校验和——FaaS 产物通常是小文件；如需大文件流式上传再演进。
func (s *S3) Put(ctx context.Context, key string, r io.Reader) (PutResult, error) {
	if err := validateKey(key); err != nil {
		return PutResult{}, err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return PutResult{}, fmt.Errorf("read object: %w", err)
	}
	sum := sha256.Sum256(data)
	checksumB64 := base64.StdEncoding.EncodeToString(sum[:])

	if _, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:         aws.String(s.bucket),
		Key:            aws.String(key),
		Body:           bytes.NewReader(data),
		ChecksumSHA256: aws.String(checksumB64),
	}); err != nil {
		return PutResult{}, fmt.Errorf("put object %q: %w", key, err)
	}
	return PutResult{Size: int64(len(data)), Sum: hex.EncodeToString(sum[:])}, nil
}

// Get 下载对象；调用方负责关闭返回的 reader。
func (s *S3) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isS3NotFound(err) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, key)
		}
		return nil, fmt.Errorf("get object %q: %w", key, err)
	}
	return out.Body, nil
}

// Delete 删除对象；S3 对不存在对象也返回成功（幂等）。
func (s *S3) Delete(ctx context.Context, key string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}); err != nil {
		return fmt.Errorf("delete object %q: %w", key, err)
	}
	return nil
}

// Stat 通过 HeadObject 获取元信息；摘要由服务端返回（base64）转换为十六进制。
func (s *S3) Stat(ctx context.Context, key string) (ObjectInfo, error) {
	if err := validateKey(key); err != nil {
		return ObjectInfo{}, err
	}
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isS3NotFound(err) {
			return ObjectInfo{}, fmt.Errorf("%w: %s", ErrNotFound, key)
		}
		return ObjectInfo{}, fmt.Errorf("stat object %q: %w", key, err)
	}

	info := ObjectInfo{}
	if out.ContentLength != nil {
		info.Size = *out.ContentLength
	}
	if out.LastModified != nil {
		info.ModTime = *out.LastModified
	}
	if out.ChecksumSHA256 != nil {
		if raw, err := base64.StdEncoding.DecodeString(*out.ChecksumSHA256); err == nil {
			info.Sum = hex.EncodeToString(raw)
		}
	}
	return info, nil
}

// Presign 生成限时下载 URL（无需凭据即可在 ttl 内下载）。
func (s *S3) Presign(key string, ttl time.Duration) (string, error) {
	req, err := s.presign.PresignGetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, func(po *s3.PresignOptions) {
		po.Expires = ttl
	})
	if err != nil {
		return "", fmt.Errorf("presign object %q: %w", key, err)
	}
	return req.URL, nil
}

// isS3NotFound 判断错误是否为「对象不存在」。
func isS3NotFound(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		code := apiErr.ErrorCode()
		return code == "NoSuchKey" || code == "NotFound"
	}
	return false
}

// 保证编译期检查：S3 实现满足 ObjectStore 接口。
var _ ObjectStore = (*S3)(nil)
