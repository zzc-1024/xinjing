# 第三方依赖与许可证声明 (THIRD PARTY NOTICES)

本项目对外部依赖的许可要求：**仅允许可商用的宽松开源许可证**
（MIT / BSD-2-Clause / BSD-3-Clause / ISC / Apache-2.0 / PostgreSQL License）。
任何 copyleft（AGPL/GPL/LGPL/MPL）或 source-available（SSPL/BSL/BUSL/RSAL）依赖一律禁止。

判断口径：以「**实际编译进发布二进制**」的依赖为准（`go list -deps`），
而非 `go list -m all` 列出的完整依赖图——图里包含 goose 等库「声明但未启用」的可选方言驱动，
它们不进入二进制，故不在此列。

---

## 直接依赖（go.mod 中 require，本仓库主动选择）

| 模块 | 版本 | 许可证 | 类型 |
|---|---|---|---|
| gorm.io/gorm | v1.31.2 | MIT | ORM |
| gorm.io/driver/postgres | v1.6.2 | MIT | PostgreSQL 驱动（GORM） |
| github.com/glebarez/sqlite | v1.11.0 | MIT | 纯 Go SQLite 驱动（GORM） |
| github.com/pressly/goose/v3 | v3.27.3 | MIT | 数据库迁移 |
| github.com/aws/aws-sdk-go-v2 | v1.43.6 | Apache-2.0 | S3 对象存储客户端 |
| github.com/aws/aws-sdk-go-v2/config | v1.32.37 | Apache-2.0 | S3 客户端配置 |
| github.com/aws/aws-sdk-go-v2/credentials | v1.19.36 | Apache-2.0 | S3 凭证 |
| github.com/aws/aws-sdk-go-v2/service/s3 | v1.107.2 | Apache-2.0 | S3 服务 |
| github.com/aws/smithy-go | v1.27.8 | Apache-2.0 | AWS SDK 基础库 |
| github.com/google/uuid | v1.6.0 | BSD-3-Clause | UUID |
| github.com/joho/godotenv | v1.5.1 | MIT | 环境变量加载 |

---

## 传递依赖（进入二进制的，按上游声明计）

以下均为宽松许可证，完整列表可由命令复现：

```bash
# 列出实际编译进 cmd/server 二进制的非标准库包
go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./cmd/server
```

按其所属模块归类的许可证：

| 模块族 | 许可证 |
|---|---|
| github.com/jackc/pgx 系列（含 puddle/pgpassfile/pgservicefile） | MIT |
| github.com/jinzhu/ 系列（inflection/now） | MIT |
| github.com/glebarez/go-sqlite | BSD-3-Clause（随 modernc 派生） |
| modernc.org/ 系列（libc/mathutil/memory/sqlite/strftime） | BSD-3-Clause |
| golang.org/x/ 系列（sync/sys/text） | BSD-3-Clause |
| go.uber.org/multierr | MIT |
| 其余小型工具库（go-humanize/go-isatty/interpolate/go-retry/bigfft） | ISC / MIT / BSD |

---

## 明确禁用（红线，禁止引入）

| 依赖 | 许可证 | 原因 |
|---|---|---|
| MinIO server | AGPL-3.0 | copyleft |
| Redis >= 7.4 | RSALv2 / SSPLv1 | source-available，有商业限制 |
| Consul | BUSL-1.1 | source-available |
| CockroachDB | BSL | source-available |
| DragonflyDB | BSL | source-available |
| go-sql-driver/mysql | MPL-2.0 | 弱 copyleft（文件级传染） |
| Garage | AGPL-3.0 | copyleft |

> 注：goose 声明支持 mysql/mssql/vertica/ydb 等方言，其对应驱动（含 MPL 的 go-sql-driver/mysql）
> 仅在启用对应方言时才被编译。本项目仅用 sqlite/postgres 方言，上述驱动**不进入二进制**；
> 若未来启用，需重新评估其许可证。

---

## 已人工核实的替代选型（详见 PLAN.md 第 7 节）

- 对象存储服务端：**RustFS**（Apache-2.0，用户已核实），替代 AGPL 的 MinIO。
- 分布式数据库：**YugabyteDB**（Apache-2.0），多主多写。
- 分布式缓存：**Valkey**（BSD-3-Clause），替代 RSALv2 的 Redis >= 7.4。
