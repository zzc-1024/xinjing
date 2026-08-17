# 心境(xinjing) FaaS 平台 — 进度与计划

Go FaaS 平台：用户上传函数、经网关调用执行，插件系统扩展平台能力。

## 当前进度

### 阶段 0 · 地基 ✅
- 统一日志（slog + source + 请求级 trace_id 关联）
- 统一 JSON 响应（`internal/response`）
- 优雅关停（Ctrl+C 不丢请求）
- 工程化（Makefile / CI / .gitattributes / go-licenses 许可证检查）

### 阶段 1 · 持久化层 ✅
- **1a 连接+迁移**：`persistence.Open`（sqlite/postgres/ydb）+ goose 内嵌迁移 + `schema_lock` 表级分布式锁 + 跨方言占位符 `rebind`（`?`→`$1`）
- **1b 对象存储**：`objectstore.ObjectStore` 接口 + local 磁盘 + s3（aws-sdk-go-v2，连 RustFS）+ sha256 内容寻址
- **1c 缓存**：`cache.Cache` 接口 + memory 实现（valkey 留后续）
- **1d 模型+仓储**：GORM 模型（UUIDv7 主键）+ 泛型 `repo.Repository[T]` + User/RefreshToken/RateLimitPolicy 仓储
  - 注：早期 APIKey 模型与仓储已被 refresh token 方案取代并移除

### 认证体系 ✅（阶段 2 的认证部分，已超原计划）
- **凭证模型**：refresh token（长期、可吊销、存哈希）+ 短期 access token（JWT，15 分钟，RS256 非对称）
- **密码**：bcrypt 加盐哈希（User.PasswordHash）
- **端点**：/register、/token、/refresh、/revoke
- **微服务拆分**：cmd/auth（认证服务，独立 auth 库 + 持私钥签发）+ cmd/server（网关，独立业务库 + 只持公钥验签）
- **密钥工具**：cmd/keygen 生成 RSA 密钥对

### 阶段 2 · API 网关层 ✅
- **认证中间件**：Authenticate（JWT 校验 + 注入 Principal）、RequireScope（权限校验 401/403）
- **限流**：令牌桶（internal/ratelimit）+ RateLimit 中间件 + 策略落库（rate_limit_policies 表）
- **路由注册表**：internal/gateway（Route + Sub 子路由 + Include 嵌套 + 命名中间件 Provider+Name 去重）
  - 中间件身份体系：Provider（官方 xinjing）+ Name，冲突策略默认报错、可配 keep-first/keep-last

### 待办阶段
- **阶段 3 · 插件系统**：Wazero Wasm 沙箱 + ABI + 能力 + 热更新（能力清单见 TODO.md，待逐项定稿）
- **阶段 4 · 插件平台**：上传 wasm/源码 → 构建 → 校验 → 注册表
- **阶段 5 · 云函数管理**：函数 CRUD/版本/路由 + 多语言 SDK + 执行器
- **管理面板 + E2E**：等核心 API 稳定后统一做

## 已锁定选型

| 组件 | 选型 | 许可证 |
|---|---|---|
| 数据库 | YugabyteDB（多主多写） | Apache-2.0 |
| ORM | GORM | MIT |
| 迁移 | pressly/goose | MIT |
| 对象存储 | RustFS（S3 协议） | Apache-2.0 |
| 缓存 | Valkey（阶段2上） | BSD-3 |
| 对象存储客户端 | aws-sdk-go-v2 | Apache-2.0 |

**许可证红线**：仅 MIT / BSD-2 / BSD-3 / ISC / Apache-2.0 / PostgreSQL License；禁止 AGPL/GPL/LGPL/MPL/SSPL/BSL/BUSL/RSAL。CI 已有 `go-licenses check` 强制校验。

## 关键约定

- 业务代码只依赖接口（ObjectStore / Cache / Repository），不直接碰 GORM/S3/缓存实现
- 主键统一 UUIDv7（`models.RegisterIDCallbacks` 全局回调生成）
- 迁移 SQL 只写 SQLite/PostgreSQL/YugabyteDB 公共子集；带参数的裸 SQL 用 `rebind` 翻译占位符
- 开发模式零外部依赖（SQLite + local disk + memory）；生产切 ydb/RustFS/Valkey 只改配置
- 测试：单元 + 集成（SQLite 真实跑）；E2E 推迟到管理面板
