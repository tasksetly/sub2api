# 工单服务实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为普通用户和管理员提供完整、权限隔离的纯文本工单处理闭环。

**Architecture:** 新增 `Ticket` 和 `TicketMessage` Ent 实体，`TicketService` 集中执行枚举校验、访问控制和状态机。Gin 处理器只处理 HTTP/认证，Vue 端使用独立 API 模块和两个受保护视图。

**Tech Stack:** Go、Ent、Gin、Google Wire、Vue 3、TypeScript、Vue I18n、Vitest、Vue Test Utils。

---

## 文件结构

| 路径 | 职责 |
| --- | --- |
| `backend/internal/domain/ticket.go` | 状态、分类、优先级、领域结构和业务错误。 |
| `backend/ent/schema/ticket.go`、`ticket_message.go` | 数据模式、用户关系与筛选索引。 |
| `backend/internal/service/ticket*.go` | 服务输入、仓储接口、状态机和用户资源隔离。 |
| `backend/internal/repository/ticket_repo.go` | Ent 映射、事务、分页和筛选。 |
| `backend/internal/handler/**/ticket_handler.go` | 用户、管理员 HTTP 接口及 DTO。 |
| `frontend/src/api/**/tickets.ts`、`types/index.ts` | 前端接口和类型。 |
| `frontend/src/views/{user,admin}/TicketsView.vue` | 用户会话与管理员队列。 |
| `router/index.ts`、`AppSidebar.vue`、`i18n/locales/**` | 双端入口和双语文案。 |

### Task 1: 建立领域和数据库模式

**Files:**
- Create: `backend/internal/domain/ticket.go`
- Create: `backend/internal/domain/ticket_test.go`
- Create: `backend/ent/schema/ticket.go`
- Create: `backend/ent/schema/ticket_message.go`
- Modify: `backend/ent/schema/user.go`
- Modify: generated `backend/ent/**`

- [ ] **Step 1: 写失败的枚举测试**

```go
func TestTicketStateHelpers(t *testing.T) {
	assert.True(t, IsTicketStatus(TicketStatusPending))
	assert.True(t, IsTicketCategory("billing"))
	assert.True(t, IsTicketPriority("urgent"))
	assert.False(t, IsTicketStatus("reopened"))
	assert.False(t, CanUserReply(TicketStatusClosed))
	assert.True(t, CanUserReply(TicketStatusResolved))
}
```

- [ ] **Step 2: 确认测试失败**

Run: `go test ./internal/domain -run TestTicketStateHelpers -count=1`

Expected: FAIL，工单状态和帮助函数未定义。

- [ ] **Step 3: 实现领域常量和 Ent 模式**

在领域层定义 `pending`、`in_progress`、`resolved`、`closed`；分类 `account`、`billing`、`api`、`usage`、`other`；优先级 `low`、`normal`、`high`、`urgent`。Ticket 定义 `UserID`、`Subject`、`Category`、`Priority`、`Status`、`LastActivityAt` 和时间字段；TicketMessage 定义 `TicketID`、`SenderUserID`、`SenderRole`、`Content`、`CreatedAt`。

Ticket 模式：主题最大 200，分类最大 32，默认优先级 `normal`、默认状态 `pending`；对 `(user_id,last_activity_at)`、`(status,last_activity_at)`、`(category,last_activity_at)`、`(priority,last_activity_at)` 建索引。TicketMessage 正文为 PostgreSQL `text`，对 `(ticket_id,created_at)` 建索引。为 User 添加 `tickets` 和 `ticket_messages` 反向边。

- [ ] **Step 4: 生成代码并重新运行测试**

Run: `go generate ./ent; go test ./internal/domain -run TestTicketStateHelpers -count=1`

Expected: Ent 代码生成成功，领域测试 PASS。

- [ ] **Step 5: 提交模式变更**

```bash
git add internal/domain/ticket.go internal/domain/ticket_test.go ent/schema/ticket.go ent/schema/ticket_message.go ent/schema/user.go ent
git commit -m "feat: add ticket persistence schema"
```

### Task 2: 实现仓储与服务状态机

**Files:**
- Create: `backend/internal/service/ticket.go`
- Create: `backend/internal/service/ticket_service.go`
- Create: `backend/internal/service/ticket_service_test.go`
- Create: `backend/internal/repository/ticket_repo.go`
- Create: `backend/internal/repository/ticket_repo_integration_test.go`
- Modify: `backend/internal/repository/wire.go`
- Modify: `backend/internal/service/wire.go`

- [ ] **Step 1: 写失败的状态机和资源隔离测试**

```go
created, err := svc.Create(ctx, 7, CreateTicketInput{
	Subject: "请求失败", Category: "api", Content: "request id: abc",
})
require.NoError(t, err)
require.Equal(t, TicketStatusPending, created.Status)

_, err = svc.GetForUser(ctx, 8, created.ID)
require.ErrorIs(t, err, ErrTicketNotFound)

_, err = svc.UpdateByAdmin(ctx, created.ID, UpdateTicketInput{Status: ptr(TicketStatusResolved)})
require.NoError(t, err)
updated, err := svc.AddUserMessage(ctx, 7, created.ID, "补充日志")
require.NoError(t, err)
require.Equal(t, TicketStatusInProgress, updated.Status)
```

另测关闭后用户和管理员均不能追加消息、非法分类/优先级被拒绝、消息写入会推进 `last_activity_at`。

- [ ] **Step 2: 确认测试失败**

Run: `go test ./internal/service -run TestTicketService -count=1`

Expected: FAIL，服务和输入类型未定义。

- [ ] **Step 3: 实现服务和仓储接口**

服务固定公开方法：

```go
Create(ctx context.Context, userID int64, in CreateTicketInput) (*TicketDetail, error)
ListForUser(ctx context.Context, userID int64, p pagination.PaginationParams) ([]Ticket, *pagination.PaginationResult, error)
GetForUser(ctx context.Context, userID, ticketID int64) (*TicketDetail, error)
AddUserMessage(ctx context.Context, userID, ticketID int64, content string) (*TicketDetail, error)
ListForAdmin(ctx context.Context, p pagination.PaginationParams, f TicketListFilters) ([]AdminTicketListItem, *pagination.PaginationResult, error)
GetForAdmin(ctx context.Context, ticketID int64) (*AdminTicketDetail, error)
AddAdminMessage(ctx context.Context, adminID, ticketID int64, content string) (*TicketDetail, error)
UpdateByAdmin(ctx context.Context, ticketID int64, in UpdateTicketInput) (*Ticket, error)
```

服务对输入 `TrimSpace`，限制主题 200、正文 10,000 字符；用户越权统一返回 `ErrTicketNotFound`。关闭工单禁止留言；用户在 `resolved` 留言自动变为 `in_progress`；管理员只能把未关闭工单设为 `in_progress`、`resolved`、`closed`。仓储使用 Ent 事务原子创建工单及首条消息，详情消息以 `created_at,id` 升序返回。

- [ ] **Step 4: 写并运行仓储集成测试**

用现有 SQLite Ent fixture 建两个用户、一张工单和多条消息，断言用户列表隔离、管理员 `status/category/priority/search` 筛选、消息排序和最后活动时间更新。

Run: `go test ./internal/service ./internal/repository -run TestTicket -count=1`

Expected: PASS。

- [ ] **Step 5: 提交服务实现**

```bash
git add internal/service/ticket.go internal/service/ticket_service.go internal/service/ticket_service_test.go internal/repository/ticket_repo.go internal/repository/ticket_repo_integration_test.go internal/repository/wire.go internal/service/wire.go
git commit -m "feat: add ticket service workflow"
```

### Task 3: 暴露 HTTP 接口并接入依赖注入

**Files:**
- Create: `backend/internal/handler/ticket_handler.go`
- Create: `backend/internal/handler/admin/ticket_handler.go`
- Create: `backend/internal/handler/dto/ticket.go`
- Create: `backend/internal/handler/ticket_handler_test.go`
- Create: `backend/internal/handler/admin/ticket_handler_test.go`
- Modify: `backend/internal/handler/handler.go`
- Modify: `backend/internal/handler/wire.go`
- Modify: `backend/internal/server/routes/user.go`
- Modify: `backend/internal/server/routes/admin.go`
- Modify: generated `backend/cmd/server/wire_gen.go`

- [ ] **Step 1: 写失败的 HTTP 契约测试**

测试认证用户 `POST /api/v1/tickets` 的成功创建、非法分类 400、读取他人工单 404、关闭后 `POST /tickets/:id/messages` 为 400。测试管理员 `GET /admin/tickets?status=pending&priority=high&search=alice` 传递筛选条件，`PATCH /admin/tickets/:id` 能更新状态/优先级，`POST /messages` 记录管理员发送者。

- [ ] **Step 2: 确认测试失败**

Run: `go test ./internal/handler/... -run TestTicket -count=1`

Expected: FAIL，路由和处理器不存在。

- [ ] **Step 3: 实现 DTO、处理器和路由**

```go
type CreateTicketRequest struct { Subject string `json:"subject" binding:"required"`; Category string `json:"category" binding:"required"`; Content string `json:"content" binding:"required"` }
type AddTicketMessageRequest struct { Content string `json:"content" binding:"required"` }
type UpdateTicketRequest struct { Status *string `json:"status"`; Priority *string `json:"priority"` }
```

用户组注册 `GET/POST /tickets`、`GET /tickets/:id`、`POST /tickets/:id/messages`；管理员组注册 `GET /admin/tickets`、`GET /admin/tickets/:id`、`POST /admin/tickets/:id/messages`、`PATCH /admin/tickets/:id`。处理器用 `middleware.GetAuthSubjectFromContext` 提取当前用户，使用 `response.ErrorFrom` 映射业务错误。

- [ ] **Step 4: 更新 Wire 并验证**

将仓储、服务和双端处理器加入 ProviderSet；在 `Handlers`、`AdminHandlers` 和两个装配函数添加 `Ticket` 字段。

Run: `go generate ./cmd/server; go test ./internal/handler/... -run TestTicket -count=1; go test ./cmd/server -run '^$'`

Expected: Wire 生成成功且所有命令 PASS。

- [ ] **Step 5: 提交 HTTP 集成**

```bash
git add internal/handler internal/server/routes/user.go internal/server/routes/admin.go cmd/server/wire_gen.go
git commit -m "feat: expose ticket management APIs"
```

### Task 4: 建立前端类型、API、入口与路由

**Files:**
- Modify: `frontend/src/types/index.ts`
- Create: `frontend/src/api/tickets.ts`
- Create: `frontend/src/api/admin/tickets.ts`
- Modify: `frontend/src/api/index.ts`
- Modify: `frontend/src/api/admin/index.ts`
- Modify: `frontend/src/router/index.ts`
- Modify: `frontend/src/components/layout/AppSidebar.vue`
- Modify: `frontend/src/i18n/locales/{zh,en}/common.ts`
- Modify: `frontend/src/i18n/locales/{zh,en}/admin/resources.ts`
- Create: `frontend/src/api/__tests__/tickets.spec.ts`
- Create: `frontend/src/router/__tests__/ticket-route.spec.ts`

- [ ] **Step 1: 写失败的 API 和路由测试**

验证用户 API 调用 `/tickets`、`/tickets/9/messages`；管理员列表带 `page`、`page_size`、`status`、`category`、`priority`、`search`；验证 `/tickets` 需要登录，`/admin/tickets` 需要管理员。

- [ ] **Step 2: 确认测试失败**

Run: `pnpm.cmd test:run -- src/api/__tests__/tickets.spec.ts src/router/__tests__/ticket-route.spec.ts`

Expected: FAIL，类型、模块和路由不存在。

- [ ] **Step 3: 实现类型/API/导航**

```ts
export type TicketStatus = 'pending' | 'in_progress' | 'resolved' | 'closed'
export type TicketCategory = 'account' | 'billing' | 'api' | 'usage' | 'other'
export type TicketPriority = 'low' | 'normal' | 'high' | 'urgent'
export interface TicketMessage { id: number; ticket_id: number; sender_user_id: number; sender_role: 'user' | 'admin'; content: string; created_at: string }
export interface TicketDetail { id: number; user_id: number; subject: string; category: TicketCategory; priority: TicketPriority; status: TicketStatus; last_activity_at: string; created_at: string; updated_at: string; messages: TicketMessage[] }
```

用户 API 导出 `list/create/getById/addMessage`，管理端导出 `list/getById/addMessage/update`，均使用已有 `apiClient`。新增 `/tickets` 与 `/admin/tickets` 懒加载路由，加入用户“我的工单”和管理员“工单管理”菜单，使用现有图标库。补齐中文/英文状态、分类、优先级、筛选、创建、回复及关闭提示。

- [ ] **Step 4: 验证并提交前端契约**

Run: `pnpm.cmd test:run -- src/api/__tests__/tickets.spec.ts src/router/__tests__/ticket-route.spec.ts; pnpm.cmd typecheck`

Expected: PASS。

```bash
git add src/types/index.ts src/api src/router/index.ts src/components/layout/AppSidebar.vue src/i18n/locales
git commit -m "feat: add ticket frontend API and navigation"
```

### Task 5: 实现用户工单工作流

**Files:**
- Create: `frontend/src/views/user/TicketsView.vue`
- Create: `frontend/src/views/user/__tests__/TicketsView.spec.ts`

- [ ] **Step 1: 写失败的用户视图测试**

模拟 `ticketsAPI.list` 返回一张 `resolved` 工单，验证列表渲染主题/状态；创建表单提交 `{ subject, category, content }`；详情回复调用 `ticketsAPI.addMessage`；`closed` 工单不存在回复输入和发送按钮。

- [ ] **Step 2: 确认测试失败并实现**

Run: `pnpm.cmd test:run -- src/views/user/__tests__/TicketsView.spec.ts`

Expected: FAIL，视图不存在。

用现有 `AppLayout`、`TablePageLayout`、`BaseDialog`、`Pagination`、`Select`、`Textarea` 组织列表、创建表单和对话详情。挂载时加载列表，点击行加载详情，成功写入后使用返回详情同步时间线；`closed` 只显示关闭状态，不渲染回复表单；失败通过 `appStore.showError(error.response?.data?.detail || t('tickets.failedToLoad'))` 告知用户。

- [ ] **Step 3: 验证并提交用户流程**

Run: `pnpm.cmd test:run -- src/views/user/__tests__/TicketsView.spec.ts`

Expected: PASS。

```bash
git add src/views/user/TicketsView.vue src/views/user/__tests__/TicketsView.spec.ts
git commit -m "feat: add user ticket workflow"
```

### Task 6: 实现管理员工单处理队列

**Files:**
- Create: `frontend/src/views/admin/TicketsView.vue`
- Create: `frontend/src/views/admin/__tests__/TicketsView.spec.ts`

- [ ] **Step 1: 写失败的管理员视图测试**

模拟 `adminAPI.tickets.list`，验证状态/分类/优先级/关键词变化后重新查询；选中行加载详情；更新优先级调用 `update(9, { priority: 'high' })`；关闭调用 `update(9, { status: 'closed' })`；回复调用 `addMessage` 后刷新详情。

- [ ] **Step 2: 确认测试失败并实现**

Run: `pnpm.cmd test:run -- src/views/admin/__tests__/TicketsView.spec.ts`

Expected: FAIL，视图不存在。

用 `AppLayout`、`TablePageLayout`、`DataTable` 创建共享队列。顶部固定状态、分类、优先级、关键词筛选；详情显示用户名称/邮箱、完整时间线、管理员状态和优先级选择、回复区。关闭时禁用管理员回复；每次状态、优先级、回复成功后刷新列表与详情。

- [ ] **Step 3: 验证并提交管理员流程**

Run: `pnpm.cmd test:run -- src/views/admin/__tests__/TicketsView.spec.ts`

Expected: PASS。

```bash
git add src/views/admin/TicketsView.vue src/views/admin/__tests__/TicketsView.spec.ts
git commit -m "feat: add admin ticket management"
```

### Task 7: 完整回归与构建验证

**Files:**
- Modify: 仅修复前述任务暴露的类型、测试或格式问题。

- [ ] **Step 1: 执行完整验证**

Run: `go test ./...`

Expected: PASS。

Run: `pnpm.cmd test:run; pnpm.cmd typecheck; pnpm.cmd build`

Expected: 三个前端命令均 PASS。

- [ ] **Step 2: 检查变更范围**

Run: `git diff --check; git status --short`

Expected: 无空白错误，仅有工单领域、生成代码、双端页面、路由/i18n 和相应测试变更。

- [ ] **Step 3: 提交仅限验证修复的收尾变更**

```bash
git add -A
git commit -m "test: verify ticket service integration"
```

仅当步骤 1-2 产生修复时执行；工作树干净时不创建空提交。

## 自检记录

- 规格要求的创建、双方回复、状态机、关闭限制、管理员筛选、优先级、双端页面、纯文本边界和测试都有对应任务。
- 不包含附件、邮件或站内通知、轮询、实时推送、个人指派、客服角色、编辑/删除消息或可配置分类。
- `TicketStatus`、`TicketCategory`、`TicketPriority`、`TicketDetail` 和服务方法名在所有任务中一致。
