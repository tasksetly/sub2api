# NewAPI 账号上游倍率问题：接续说明

## 用户需求

NewAPI 对应的本地账号看不到对应的上游倍率，即使已经配置了倍率。NewAPI 的上游倍率可以通过上游账号同步到本地账号。

用户明确要求：不要编译，不要运行 Go 测试，不要运行前端构建，避免电脑卡死。

## 当前状态

截至本文档创建时：

- 已完成代码检索和根因定位。
- 尚未修改业务代码。
- 尚未运行编译、`go test`、前端构建或其他高负载检查。
- 当前工作目录：`/home/debian/ai/sub2api-deploy/src/sub2api`

## 根因

当前 NewAPI 的上游同步逻辑已经能读取 NewAPI 的分组倍率：

- `GET /api/user/self/groups` 或 `GET /api/user/groups`
- 每个分组对象中的 `ratio` 被解析为 `NewAPIGroupInfo.Ratio`
- `backend/internal/service/upstream_provider_service.go` 中的 `syncProvider` 把它存进 `upstream_groups.rate_multiplier`

当前存在两个断点：

1. NewAPI 同步后的 `ratio` 只存到了 `upstream_groups` 分组快照，没有按照 `accounts.upstream_provider_id + accounts.upstream_remote_group_id` 回填账号倍率。
2. 账号列表的“上游声明倍率”列读取的是账号 `extra.upstream_billing_probe` 快照。这个快照依赖 `/v1/sub2api/billing`，而 NewAPI 不提供该 Sub2API 专用端点，所以 NewAPI 账号不会产生可展示的探测快照，页面显示“未探测”或空值。

相关现有代码：

- NewAPI 分组 ratio 读取：
  - `backend/internal/service/upstream_provider_client.go`
  - `ListNewAPIGroups`
- NewAPI 分组快照保存：
  - `backend/internal/service/upstream_provider_service.go`
  - `syncProvider` 的 `provider.ProviderType == UpstreamProviderTypeNewAPI` 分支
- 上游建号时首次设置账号倍率：
  - `backend/internal/service/upstream_provider_service.go`
  - `createLocalAccount`
  - 当前使用 `group.ComparableRate() * NormalizeRateCorrection(provider.RateCorrection)`
- 账号列表回填上游名称：
  - `backend/internal/handler/admin/account_upstream_provider.go`
  - `enrichUpstreamProviders`
- 账号列表 DTO 映射：
  - `backend/internal/handler/dto/mappers.go`
  - `AccountFromServiceShallow`
  - `AccountListItemFromAccount`
- 前端账号列表：
  - `frontend/src/views/admin/AccountsView.vue`
- 前端上游倍率单元格：
  - `frontend/src/components/account/UpstreamBillingRateCell.vue`
- 前端类型：
  - `frontend/src/types/index.ts`
  - `UpstreamBillingProbeSnapshot`
  - `Account`
  - `AccountListItem`

## 建议实施方案

### 方案目标

让 NewAPI 账号能够：

- 在账号列表中显示对应 NewAPI 分组的 `ratio`。
- 上游同步后，按上游账号配置的充值比例修正系数更新本地账号倍率。
- 继续保留现有同步开关语义：只有启用了上游倍率同步的账号才自动回写 `accounts.rate_multiplier`。
- 已有 NewAPI 账号和之后新建的 NewAPI 账号都能生效。
- 不依赖 `/v1/sub2api/billing`。

### 推荐改法

#### 1. 给账号列表响应增加 NewAPI 上游倍率信息

在后端账号列表回填阶段，使用账号的：

- `UpstreamProviderID`
- `UpstreamRemoteGroupID`

批量查询对应的 `upstream_groups`，再把匹配到的倍率放到账号响应中。

建议增加明确的管理员字段，例如：

```go
NewAPIUpstreamRateMultiplier *float64 `json:"newapi_upstream_rate_multiplier,omitempty"`
NewAPIUpstreamRateUpdatedAt *time.Time `json:"newapi_upstream_rate_updated_at,omitempty"`
```

也可以复用已有 `extra.upstream_billing_probe` 的展示结构，但不建议伪造 probe 快照，因为 NewAPI 的来源是分组同步，不是 `/v1/sub2api/billing` 探测。更清晰的做法是增加专门的上游分组倍率字段。

需要同步修改：

- `backend/internal/handler/dto/types.go` 中的 `Account`
- `backend/internal/handler/dto/types.go` 中的 `AccountListItem`
- `backend/internal/handler/dto/mappers.go` 中的映射函数
- `backend/internal/service/account.go` 中的 `Account`，或在 handler 层使用专用的列表增强结构
- `frontend/src/types/index.ts` 中的 `Account` 和 `AccountListItem`
- `frontend/src/views/admin/AccountsView.vue`

#### 2. 后端批量回填匹配的 NewAPI 分组倍率

建议在 `backend/internal/handler/admin/account_upstream_provider.go` 增加类似 `enrichUpstreamProviderRates` 的逻辑，与 `enrichUpstreamProviders` 一样在账号列表返回前执行。

不要逐账号查询。应当：

1. 收集当前页所有账号的 `(upstream_provider_id, upstream_remote_group_id)`。
2. 一次 SQL 查询 `upstream_groups`。
3. 将 `rate_multiplier`、`effective_rate_multiplier`、`synced_at` 回填到对应账号。
4. NewAPI 使用基础 `rate_multiplier`，因为 NewAPI 当前同步逻辑没有用户专属倍率覆盖字段。
5. 如要展示经过充值修正后的本地成本倍率，可额外回填 `rate_multiplier * NormalizeRateCorrection(provider.rate_correction)`，但页面需要明确它是“上游倍率”还是“本地修正后倍率”。

仓储层可以新增一个按复合键批量查询的方法，例如：

```go
type UpstreamGroupRateLookup struct {
    ProviderID     int64
    RemoteGroupID  int64
    Name            string
    RateMultiplier  float64
    SyncedAt        time.Time
}
```

然后在 `UpstreamProviderRepository` 增加批量查询接口；或者如果只想减少接口改动，也可以在已有上游服务中添加批量方法。

#### 3. 上游同步时回写已经存在的 NewAPI 账号倍率

在 `backend/internal/service/upstream_provider_service.go` 的 NewAPI `syncProvider` 分支中：

- 构造 `snapshotGroups` 时记录每个分组的 `ratio`。
- `ReplaceGroups` 成功后，根据 `provider.ID + remote_group_id` 找出本地账号。
- 对已开启 `upstream_billing_rate_sync_enabled` 的账号，按：

```text
本地账号倍率 = NewAPI 分组 ratio × NormalizeRateCorrection(provider.RateCorrection)
```

进行更新。

必须遵守现有约束：

- 只更新明确开启倍率同步的账号。
- 不覆盖手工设置且未开启同步的账号。
- 倍率写回前沿用已有自动同步的合法范围和精度限制，避免 0 或异常值破坏账号计费。
- 更新账号倍率时应通过现有账号仓储/服务能力，确保调度缓存和分组绑定关系正确刷新。
- 如果批量更新能力不足，可以新增专门的仓储方法，例如按 provider 和 remote group 批量同步 NewAPI 账号倍率。

目前 `AccountRepository` 已有普通更新能力，但需要确认如何安全地按账号 Extra 中的同步开关进行批量更新。不要直接绕过既有调度快照/绑定清理逻辑的 SQL，除非新方法完整复用这些机制。

#### 4. 新创建的 NewAPI 账号

现有 `createLocalAccount` 已经设置：

```go
correctedRate := group.ComparableRate() * NormalizeRateCorrection(provider.RateCorrection)
RateMultiplier: &correctedRate
```

并且默认启用：

- `upstream_billing_probe_enabled`
- `upstream_billing_rate_sync_enabled`

这部分逻辑需要保留。由于 NewAPI 不适合走 `/v1/sub2api/billing`，建议新增 NewAPI 专用同步标记或在账号上游来源字段存在时使用上游分组同步作为倍率来源，避免 NewAPI 账号的现有“probe enabled”开关造成误导。

可选改进：

- 在前端 NewAPI 账号上显示“来自 NewAPI 分组同步”。
- 不要求 NewAPI 账号执行 Sub2API billing probe。
- 如果现有 UI 复用开关，至少在 NewAPI 账号上把提示文案改成同时涵盖“上游分组同步”。

## 需要重点注意的现有行为

### `/v1/sub2api/billing` 探测逻辑

文件：`backend/internal/service/upstream_billing_probe.go`

该逻辑访问：

```text
/v1/sub2api/billing
```

并解析 `sub2api.key_billing` 响应。NewAPI 不提供该端点，因此不能只修改前端状态判断来解决问题。前端如果只读取 probe 快照，NewAPI 永远看不到 ratio。

### 账号倍率同步的现有保护

现有自动写回逻辑位于：

- `backend/internal/service/upstream_billing_probe.go`
- `backend/internal/repository/account_repo.go`

它会检查：

- `upstream_billing_probe_enabled == true`
- `upstream_billing_rate_sync_enabled == true`
- 倍率范围和精度
- 账号身份是否在探测期间发生变化
- 账号分组绑定是否因倍率变化而失效
- 调度缓存/outbox 是否需要刷新

NewAPI 的同步实现应该复用这些保护原则。

### NewAPI 的远程分组 ID

NewAPI 使用分组名称作为远程键，本地通过稳定 hash 生成 `remote_group_id`：

```go
stableNewAPIGroupID(group.Name)
```

所以账号关联应使用：

```text
accounts.upstream_provider_id = upstream_groups.upstream_provider_id
accounts.upstream_remote_group_id = upstream_groups.remote_group_id
```

不能直接拿 NewAPI 的分组字符串与账号字段比较。

## 建议实施顺序

1. 先查看 `Account`、`AccountListItem`、`UpstreamProviderRepository` 的完整接口和仓储实现，确定新增字段放在 service 层还是 handler 层。
2. 新增批量读取上游分组倍率的方法。
3. 在账号列表 handler 中回填 NewAPI 分组倍率。
4. 在 NewAPI `syncProvider` 中调用批量同步账号倍率的方法。
5. 修改前端类型和 `UpstreamBillingRateCell.vue`，优先显示 NewAPI 分组倍率。
6. 增加必要的低成本单元测试或静态检查；遵守用户要求，不编译、不运行测试、不运行构建。
7. 用 `git diff --check`、`rg` 和人工 diff 检查确认代码格式与引用完整性。`git diff --check` 是低开销静态检查，如果环境仍不稳定也可以跳过。

## 当前尚未完成的代码文件

预计会涉及：

- `backend/internal/service/account.go`
- `backend/internal/service/account_service.go`
- `backend/internal/service/upstream_provider.go`
- `backend/internal/service/upstream_provider_service.go`
- `backend/internal/repository/account_repo.go` 或 `backend/internal/repository/upstream_provider_repo.go`
- `backend/internal/handler/admin/account_handler.go`
- `backend/internal/handler/admin/account_upstream_provider.go`
- `backend/internal/handler/dto/types.go`
- `backend/internal/handler/dto/mappers.go`
- `frontend/src/types/index.ts`
- `frontend/src/components/account/UpstreamBillingRateCell.vue`
- `frontend/src/views/admin/AccountsView.vue`
- 对应的中英文 i18n 文案文件

具体文件应以继续排查后的最小改动为准，不要机械地全部修改。

## 禁止事项

- 不要执行 `go build`、`go test`、`go generate`。
- 不要执行 `pnpm build`、`pnpm test`、`npm run build` 或其他前端构建/测试。
- 不要启动服务或 Docker 编译流程。
- 不要修改数据库迁移，除非确认现有字段无法完成需求；当前判断不需要迁移。
- 不要把 NewAPI ratio 伪装成 `/v1/sub2api/billing` 的有效响应。
- 不要覆盖未开启上游倍率同步的账号手工倍率。

## 接续时的第一步

恢复环境后先读取本文件，然后执行低开销的只读命令确认工作区状态：

```bash
pwd
git status --short
rg -n "type Account struct|type AccountListItem struct|type UpstreamProviderRepository|func \(s \*UpstreamProviderService\) syncProvider" backend/internal backend/ent frontend/src
```

确认没有其他并行修改后，再按“建议实施顺序”继续。