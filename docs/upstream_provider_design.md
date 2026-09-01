# 上游 sub2api 供应商管理 — 设计文档

## 1. 背景与目标

本地 sub2api 的上游也是 sub2api 实例。上游数量多时，逐个登录他们的站点做比价、
查余额、建 Key 非常低效。

目标：在本地管理端集中管理所有上游，做到

1. 存上游后台凭据，自动登录拿 access/refresh token
2. access JWT 到期前优先用 refresh token 换发新 token，失败再用密码重新登录
3. 拉取上游的分组（含倍率、限额）、账户余额、并发限制
4. 直接在上游创建 API Key，并落地成本地账户，无需去上游页面
5. 一张表横向比价：分组倍率 + 限额、余额 + 并发、本地实际用量成本

## 2. 关键设计决策

### 2.1 为什么不复用已有的 upstream_billing_probe

代码库里已有 `internal/service/upstream_billing_probe.go`（1136 行），它用
**账号自带的 API Key** 探 `/v1/sub2api/billing`。两者是互补关系，不重叠：

| | upstream_billing_probe（已有） | 本功能（新增） |
|---|---|---|
| 凭据 | 账号的 API Key | 后台账号密码或 access/refresh token |
| 能看到 | 该 Key 已绑定分组的倍率 | 全部可用分组 + 余额 + 并发 |
| 能否建 Key | 否 | 是 |
| 前提 | 账号已存在 | 账号还不存在 |

所以本功能沿用它的约定（超时、响应体上限、错误分类口径），但独立实现。

### 2.2 上游接口来源

上游就是 sub2api，因此不需要猜接口，直接读本仓库的用户侧路由：

| 用途 | 接口 | 本仓库位置 |
|---|---|---|
| 登录 | `POST /api/v1/auth/login` | `internal/handler/auth_handler.go:238` |
| JWT 续期 | `POST /api/v1/auth/refresh` | `internal/handler/auth_handler.go:685` |
| 2FA | `POST /api/v1/auth/login/2fa` | `internal/handler/auth_handler.go:300` |
| 余额 + 并发 | `GET /api/v1/user/profile` | `internal/server/routes/user.go:31` |
| 分组（倍率/限额） | `GET /api/v1/groups/available` | `internal/server/routes/user.go:88` |
| 专属倍率覆盖 | `GET /api/v1/groups/rates` | `internal/server/routes/user.go:89` |
| 创建 API Key | `POST /api/v1/keys` | `internal/server/routes/user.go:80` |

### 2.3 会话凭据必须可逆加密

access JWT 不能在本地重新签名：签名密钥属于上游。正确的续期方式是调用上游
`/api/v1/auth/refresh`，用 refresh token 换发新的 access/refresh token；只有 refresh
失败时才用原密码重新登录。因此 password、access token、refresh token 都需要可逆加密。
复用已有的 `internal/repository/aes_encryptor.go`（AES-256-GCM，实现
`service.SecretEncryptor`，密钥取自 `cfg.Totp.EncryptionKey`）。加解密边界收在仓储层：
service 层拿到的凭据始终是明文，DTO 层则完全不回传。refresh token 轮转时必须保存
上游返回的新值；旧 refresh token 不应继续使用。

### 2.4 accounts 加外键，而不是靠 supplier 字符串匹配

比价要把「上游声明的倍率」和「本地实际花了多少钱」放一起看，后者来自本地
`usage_logs`。用 `accounts.upstream_provider_id` 外键关联，比 `supplier` 字符串
匹配可靠：上游改名不会断开关联。`ON DELETE SET NULL` —— 删上游不该连带删掉还在
用的账号。

### 2.5 只读同步，不自动写回本地账号

同步到的并发/倍率**仅用于展示比价**，不自动改本地 `accounts.concurrency` /
`rate_multiplier`。理由：上游改倍率不该让本地调度和计费口径悄悄变。管理员看到
差异后自行决定是否应用。

### 2.6 新建账号默认不进默认分组

`SkipDefaultGroupBind: len(input.LocalGroupIDs) == 0` —— 没勾本地分组时不要自动
绑到平台默认分组，否则倍率还没核对过的新账号会立刻参与真实流量调度。

## 3. 数据模型

### upstream_providers

存能登录上游后台的账号。密码/TOTP/access token/refresh token 均为 AES-256-GCM 密文。

关键字段：`base_url`、`username`、`password_encrypted`、`totp_secret_encrypted`、
`token_encrypted`、`refresh_token_encrypted` + `token_expires_at`（会话缓存）、`balance`、
`frozen_balance`、`upstream_concurrency`（同步快照）、`last_sync_at`、`last_sync_error`、
`sync_enabled`。新增字段迁移为 `backend/migrations/224_upstream_provider_refresh_token.sql`。

### upstream_groups

上游分组的只读镜像，每次同步整体覆盖。按 `(provider_id, remote_group_id)` upsert
而非先删后插 —— 直接 delete-then-insert 会让 id 每轮同步都变，前端勾选状态失效。

关键字段：`remote_group_id`（建 Key 时要回传给上游）、`rate_multiplier`、
`effective_rate_multiplier`（叠加专属倍率后的实际值，**这才是该用来比价的**）、
日/周/月限额。

### accounts（改动）

新增可空 `upstream_provider_id` + 索引。

迁移：`backend/migrations/221_upstream_providers.sql`

## 4. 分层实现

```
ent/schema/upstream_provider.go      实体定义
ent/schema/upstream_group.go
migrations/221_upstream_providers.sql
migrations/224_upstream_provider_refresh_token.sql

internal/service/upstream_provider.go          领域模型 + 仓储接口
internal/service/upstream_provider_client.go   上游 HTTP 客户端
internal/service/upstream_provider_service.go  登录/同步/建号编排
internal/service/upstream_provider_runner.go   30 分钟定时同步
internal/repository/upstream_provider_repo.go  持久化（含加解密边界）
internal/handler/dto/upstream_provider.go      响应 DTO
internal/handler/admin/upstream_provider_handler.go
```

### 管理端路由

挂在 `/api/v1/admin/upstream-providers`（`internal/server/routes/admin.go`）：

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `` | 列表（余额/并发/倍率区间/本地成本） |
| POST | `` | 新增 |
| GET/PUT/DELETE | `/:id` | 查/改/删 |
| POST | `/:id/test` | 测试连接 |
| POST | `/:id/sync` | 手动同步 |
| GET | `/:id/groups` | 分组快照（供勾选） |
| POST | `/:id/provision` | 勾选分组 → 建 Key → 落地账号 |

## 5. 安全设计

- **凭据不回显**：DTO 只给 `has_password` / `has_totp_secret` / `has_refresh_token`
  布尔位。回显等于给任何拿到管理端会话的人一份明文凭据。改密码只能重填。
- **SSRF 防护**：`base_url` 由管理员输入，复用 `validateUpstreamBaseURL` 同一套
  口径（`urlvalidator` + allowlist + 私网校验）。
- **响应体上限** 512KB，**超时** 15s，防止上游拖死同步。
- **解密失败降级**：一个坏密文只记日志、留空值，不让整张列表打不开。
- **多实例防重**：定时同步用 `tryAcquireSingletonLeaderLock`（Redis 优先，
  回退 PG advisory lock）。锁 TTL 10 分钟 > 一轮同步最坏耗时。

## 6. 失败处理

登录失败必须区分三种情况，否则管理员无法判断该怎么修：

| 情况 | 错误码 | 可操作性 |
|---|---|---|
| 上游开了验证码 | `UPSTREAM_PROVIDER_CAPTCHA_REQUIRED` | 自动登录彻底不可行，需手动 token |
| 上游开了 2FA 但本地没存密钥 | `UPSTREAM_PROVIDER_TOTP_REQUIRED` | 补个 TOTP 密钥即可 |
| 密码错 | `UPSTREAM_PROVIDER_UNAUTHORIZED` | 重填密码 |
| refresh token 失效 | 上游认证错误 | 有密码时自动回退登录；无密码时需重新粘贴 token |

access JWT 临近过期时优先调用 `/api/v1/auth/refresh`。refresh token 失效时，有密码的上游
自动回退密码登录并覆盖旧会话；只有手工 token 的上游没有可用的续期凭据，只能重新粘贴。
失败原因落 `last_sync_error`，前端直接展示。

其他：
- `GET /groups/rates` 在老版本上游可能不存在 → 404 不算错误，回退基础倍率
- 批量建号逐分组独立处理，某个失败不影响其他；已建好的 Key 不回滚
  （上游多一个闲置 Key 无害，回滚需额外删除权限且可能半途失败）

## 7. 当前状态

已完成并通过 `go build ./...`：

- [x] Ent schema + `go generate ./ent`
- [x] SQL 迁移 221 + 224（refresh token）
- [x] 上游 HTTP 客户端（登录/refresh/4 个管理接口 + 2FA + 错误分类）
- [x] 仓储层（含加解密、本地成本聚合、倍率区间）
- [x] 服务层（登录/refresh token 续期/token 缓存/同步/建号）
- [x] DTO + 管理端 Handler + 路由
- [x] Wire DI 全链路（`cmd/server/wire_gen.go:226-228`）
- [x] 30 分钟定时同步 runner（leader lock 防重）

待完成：

- [ ] runner 注册进 Wire 的 cleanup 链（`Stop()` 优雅退出）
- [x] 前端上游管理页（列表/新增/编辑/测试/同步/勾选建号）+ i18n + TS 类型
- [x] 上游登录/refresh/手工 token 单元测试
- [ ] `go test -tags=unit ./...` 与 `golangci-lint run ./...`

## 8. 环境注意

`go.mod` 要求 Go 1.26.5，但系统装的是 1.25.9 且 `GOTOOLCHAIN=local`。
本次开发用 `GOTOOLCHAIN=go1.26.5` 前缀执行命令（自动下载工具链到模块缓存，
不动系统安装）。DEV_GUIDE.md 里写的 1.25.7 已过时。
