# AGENT.md — 后续模型协作须知

## 项目
心境(xinjing)：Go FaaS 平台。模块名 `xinjing`，Go 1.26。

## 说话方式（重要）
- 用户**不懂 Go 语言**，是初学者。每次写/改代码后，要**逐个文件讲解**，讲清每行/每段的作用，不省略、不跳步。
- 解释 Go 语法概念（如接口、泛型、`defer`、结构体嵌入、`errors.Is`）时，用通俗类比，别假设他懂。
- 用中文回复。
- 主动说明「为什么这么做」，尤其是踩坑后的结论。

## 绝对红线
- **许可证**：只允许 MIT / BSD-2 / BSD-3 / ISC / Apache-2.0 / PostgreSQL License。禁止 AGPL/GPL/LGPL/MPL/SSPL/BSL/BUSL/RSAL。新增任何依赖前先核对其 LICENSE 文件。
- **不改用户的全局 Go 环境**：不擅自 `go env -w`。需要临时改环境时用当前会话的 `$env:` 变量，用完清理。

## 沙箱环境事实（Windows PowerShell）
- 默认 GOCACHE / GOPATH（`C:\Users\Zzc1024\...`）**不可写**。跑 `go build/test/vet/get` 前必须设：
  ```powershell
  $env:GOCACHE = "D:\projects\xinjing\.gocache"
  $env:GOPATH = "D:\projects\xinjing\.gopath"
  ```
  用完删掉临时目录。
- 磁盘缓存这些目录已在 `.gitignore` 忽略，别提交。

## 已确定的架构决策（不要推翻重来）
- 数据库 YugabyteDB（PG 协议）、ORM GORM、迁移 goose、对象存储 RustFS（S3 协议）、缓存 Valkey。
- 开发模式零外部依赖：SQLite(`glebarez/sqlite`) + local disk + memory cache。
- 主键 UUIDv7，由 `models.RegisterIDCallbacks` 全局回调生成（注意：GORM **不会**递归调用嵌入结构体的 `BeforeCreate`，所以必须用全局回调，别改回嵌入式钩子）。
- 带参数的裸 SQL 用 `migrate.rebind()` 翻译占位符（SQLite `?`，PG `$1`）。

## 测试约定
- 做单元 + 集成测试（SQLite 真实跑）；**E2E 推迟到管理面板完成后**，现在别碰。
- 测试结束要关闭 SQLite 连接（`t.Cleanup` 里 `sqlDB.Close()`），否则 TempDir 清理会失败。

## 已知坑（别再踩）
- `vendor/` 目录会被用户 VS Code 自动生成且不一致，导致 go 命令报 `inconsistent vendoring`。用 `$env:GOFLAGS = "-mod=mod"` 绕过，别纠结 vendor。
- `go test` 里 `record not found` 日志是「故意查不存在记录」的预期行为，不是错误。

## 主线顺序
阶段 0、1 已全部完成。接下来是：阶段 2 网关层 → 3 插件 → 4 插件平台 → 5 云函数 → 管理面板+E2E。动手前先给用户简短计划确认。
