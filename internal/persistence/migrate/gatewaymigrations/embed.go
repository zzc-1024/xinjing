// Package gatewaymigrations 通过 go:embed 内嵌网关服务的数据库迁移 SQL 文件。
// 网关服务（cmd/server）独立维护业务数据库：functions、routes、plugins、invocation_logs。
package gatewaymigrations

import "embed"

// FS 包含网关服务的全部 *.sql 迁移文件。
//
//go:embed *.sql
var FS embed.FS
