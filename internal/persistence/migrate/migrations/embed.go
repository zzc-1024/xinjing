// Package migrations 通过 go:embed 内嵌数据库迁移 SQL 文件。
package migrations

import "embed"

// FS 包含全部 *.sql 迁移文件，供 goose 使用。
//
//go:embed *.sql
var FS embed.FS
