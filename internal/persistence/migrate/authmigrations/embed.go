// Package authmigrations 通过 go:embed 内嵌认证服务的数据库迁移 SQL 文件。
// 认证服务（cmd/auth）独立维护 auth 数据库：users、refresh_tokens。
package authmigrations

import "embed"

// FS 包含认证服务的全部 *.sql 迁移文件。
//
//go:embed *.sql
var FS embed.FS
