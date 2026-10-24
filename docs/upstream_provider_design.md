# 上游管理（sub2api / NewAPI）— 设计文档

本功能同时支持两种协议，`upstream_providers.provider_type` 区分：

- `sub2api`：后台账号密码或 sub2api JWT，使用 `/api/v1/*` 管理接口。
- `newapi`：使用用户在 NewAPI 后台自行生成的个人 API token（通常为 `sk-`）作为 Bearer token。它不是 JWT，也不需要账号、密码、TOTP 或 refresh token；该 token 可查询自身额度并直接作为本地账号凭据。

NewAPI 的分组是字符串名称而不是数字 ID，名称保存在 `upstream_groups.remote_group_name`，`remote_group_id` 使用名称的稳定哈希以兼容现有建号接口和前端选择模型。NewAPI API token 不访问 dashboard，分组从公开的 `/api/user/groups` 读取，余额来自 `/api/usage/token/`，按 NewAPI 默认的 `500000 quota = $1` 转换。

NewAPI 管理地址统一按站点根地址保存；如果输入地址以 `/v1` 结尾，服务层会去掉该后缀，避免管理接口变成 `/v1/api/...`。只有落地本地 OpenAI 账号时才在凭据 `base_url` 末尾追加 `/v1`。

## 1. 背景与目标

本地 sub2api 的上游也是 sub2api 实例。上游数量多时，逐个登录他们的站点做比价、
查余额、建 Key 非常低效。

目标：在本地管理端集中管理所有上游，做到

1. sub2api 使用后台账号密码或已有 JWT；NewAPI 直接使用用户自己生成的个人 API token
2. sub2api access JWT 到期前优先用 refresh token 换发新 token；NewAPI 不进行登录或 refresh
3. 拉取上游的分组（含倍率、限额）、账户余额、并发限制
4. sub2api 可直接在上游创建 API Key；NewAPI 复用用户提供的个人 API token 作为本地账号凭据
5. 一张表横向比价：分组倍率 + 限额、余额 + 并发、本地实际用量成本

## 2. 关键设计决策

### 2.1 为什么不复用已有的 upstream_billing_probe

代码库里已有 `internal/service/upstream_billing_probe.go`（1136 行），它用
**账号自带的 API Key** 探 `/v1/sub2api/billing`。两者是互补关系，不重叠：

| | upstream_billing_probe（已有） | 本功能（新增） |
|---|---|---|
| 凭据 | 账号的 API Key | sub2api 后台账号密码/JWT；NewAPI 用户个人 API token |
| 能看到 | 该 Key 已绑定分组的倍率 | 全部可用分组 + 余额 + 并发 |
| 能否建 Key | 否 | sub2api 可以；NewAPI 直接复用用户提供的 API token |
| 前提 | 账号已存在 | sub2api 账号还不存在；NewAPI token 已由用户在后台生成 |

所以本功能沿用它的约定（超时、响应体上限、错误分类口径），但独立实现。NewAPI 只使用用户手工生成的个人 API token，不尝试用账号密码登录，也不把它解析或当作 JWT 使用。

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

NewAPI 使用用户个人 API token，不经过 sub2api 的 JWT 登录流程，也不调用 token 管理端点：

| 用途 | 接口 | 说明 |
|---|---|---|
| token 用量/余额 | `GET /api/usage/token/` | API token 的权威额度接口，按 `quota / 500000` 转换 |
| 可用分组 | `GET /api/user/groups` | 公开分组名称和倍率，不需要 dashboard 登录 |
| 本地账号凭据 | 用户提供的 `sk-` token | NewAPI API token 直接复用，不在本地或远程再次创建 |

NewAPI 的 token usage envelope 返回 `success` 或布尔 `code`；客户端同时兼容这些格式和 sub2api 的数字 `code` envelope。

### 2.3 凭据必须可逆加密

sub2api 的 access JWT 不能在本地重新签名... 因此 sub2api 的 password、access token、refresh token 都需要可逆加密。
NewAPI 不登录、不 refresh，只保存用户在其后台生成的个人 API token；该 token 同样按 AES-256-GCM 加密存储。
复用已有的 `internal/repository/aes_encryptor.go`（AES-256-GCM，实现
`service.SecretEncryptor`，密钥取自 `cfg.Totp.EncryptionKey`）。加解密边界收在仓储层：
service 层拿到的凭据始终是明文，DTO 层则完全不回传。

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

新增 `provider_type`，历史记录默认 `sub2api`；NewAPI 可不填 `username`，token 仍按 AES-256-GCM 加密存储。

关键字段：`base_url`、`username`、`password_encrypted`、`totp_secret_encrypted`、
`token_encrypted`、`refresh_token_encrypted` + `token_expires_at`（会话缓存）、`balance`、
`frozen_balance`、`upstream_concurrency`（同步快照）、`last_sync_at`、`last_sync_error`、
`sync_enabled`。新增字段迁移为 `backend/migrations/246_newapi_upstream_providers.sql`。

### upstream_groups

新增 `remote_group_name` 保存 NewAPI 的真实字符串分组键。上游分组的只读镜像，每次同步整体覆盖。按 `(provider_id, remote_group_id)` upsert
而非先删后插 —— 直接 delete-then-insert 会让 id 每轮同步都变，前端勾选状态失效。

关键字段：`remote_group_id`（兼容现有选择/存储模型；NewAPI 使用稳定哈希）、`remote_group_name`、`rate_multiplier`、
`effective_rate_multiplier`（叠加专属倍率后的实际值，**这才是该用来比价的**）、
日/周/月限额。

### accounts（改动）

新增可空 `upstream_provider_id` + 索引。

迁移：`backend/migrations/221_upstream_providers.sql`、`backend/migrations/246_newapi_upstream_providers.sql`
## 4. 分层实现

```
ent/schema/upstream_provider.go      实体定义
ent/schema/upstream_group.go
migrations/221_upstream_providers.sql
migrations/224_upstream_provider_refresh_token.sql
migrations/246_newapi_upstream_providers.sql

internal/service/upstream_provider.go          领域模型 + 仓储接口
internal/service/upstream_provider_client.go   上游 HTTP 客户端
internal/service/upstream_provider_service.go  sub2api 登录/refresh 与 NewAPI token-only 同步/建号编排
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
| 上游开了验证码（sub2api） | `UPSTREAM_PROVIDER_CAPTCHA_REQUIRED` | 自动登录彻底不可行，需手动 token |
| 上游开了 2FA 但本地没存密钥（sub2api） | `UPSTREAM_PROVIDER_TOTP_REQUIRED` | 补个 TOTP 密钥即可 |
| 密码错（sub2api） | `UPSTREAM_PROVIDER_UNAUTHORIZED` | 重填密码 |
| NewAPI 个人 API token 无效/被撤销 | `UPSTREAM_PROVIDER_UNAUTHORIZED` | 在 NewAPI 后台重新生成个人 API token 并替换；不尝试账号密码登录或 refresh |
| refresh token 失效（sub2api） | 上游认证错误 | 有密码时自动回退登录；无密码时需重新粘贴 token |

access JWT 临近过期时优先调用 `/api/v1/auth/refresh`。refresh token 失效时，有密码的 sub2api 上游
自动回退密码登录并覆盖旧会话；NewAPI 个人 API token 不解析为 JWT，也没有登录/refresh 续期流程。
失败原因落 `last_sync_error`，前端直接展示。

其他：
- `GET /groups/rates` 在老版本上游可能不存在 → 404 不算错误，回退基础倍率
- 批量建号逐分组独立处理，某个失败不影响其他；已建好的 Key 不回滚
  （上游多一个闲置 Key 无害，回滚需额外删除权限且可能半途失败）

## 7. 当前状态

当前实现状态（按约束未执行构建、自动化测试或 lint）：

- [x] Ent schema + `go generate ./ent`
- [x] SQL 迁移 221 + 246（NewAPI provider type、token-only 凭据和命名分组）
- [x] 上游 HTTP 客户端（sub2api 登录/refresh/4 个管理接口 + 2FA；NewAPI token quota/分组接口）
- [x] 仓储层（含加解密、本地成本聚合、倍率区间）
- [x] 服务层（sub2api 登录/refresh token 续期；NewAPI token-only 同步/建号）
- [x] DTO + 管理端 Handler + 路由
- [x] Wire DI 全链路（`cmd/server/wire_gen.go:226-228`）
- [x] 30 分钟定时同步 runner（leader lock 防重）

待完成：

- [ ] runner 注册进 Wire 的 cleanup 链（`Stop()` 优雅退出）
- [x] 前端上游管理页（列表/新增/编辑/测试/同步/勾选建号）+ i18n + TS 类型
- [x] 上游登录/refresh/手工 token 单元测试
- [x] NewAPI token quota、用量回退、分组同步客户端测试
- [ ] `go test -tags=unit ./...` 与 `golangci-lint run ./...`

## 8. 环境注意

本次仅进行了代码阅读、静态逻辑检查、格式化和 diff 检查，未执行构建、自动化测试或 lint。
