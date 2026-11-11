# Tasksetly 项目开发文档

> 这份文档是给 Claude Code 看的快速参考，开发前先读这里，减少重复探索。

## 快速定位

| 文件 | 用途 |
|------|------|
| **CLAUDE_DEV.md** | 本文档，开发前必读 |
| **AGENTS.md** | Git 工作流规则：功能完成后提交，不编译，只提交相关文件 |
| **DEV_GUIDE.md** | 环境配置和 CI 要求 |
| **README.md** | 项目介绍（用户视角） |

---

## 项目结构

```
sub2api/
├── backend/                    # Go 后端（Gin + Ent ORM）
│   ├── cmd/
│   │   ├── server/            # 主服务入口
│   │   ├── jwtgen/            # JWT 生成工具
│   │   ├── profit-preview/    # 利润预览工具
│   │   └── cleanup-ingress-reject-logs/  # 日志清理
│   ├── ent/                   # Ent ORM 生成代码（不要手动改，改 schema 后 go generate）
│   │   └── schema/            # 数据库 schema 定义（45+ 表）
│   ├── internal/
│   │   ├── config/            # 配置加载
│   │   ├── domain/            # 领域模型和常量（枚举、配置结构）
│   │   ├── handler/           # HTTP handlers（按模块分包）
│   │   │   └── admin/         # 管理员专用 handlers
│   │   ├── middleware/        # Gin 中间件（auth、CORS、rate limit）
│   │   ├── model/             # 请求/响应 DTO
│   │   ├── repository/        # 数据库访问层（读写封装）
│   │   ├── service/           # 业务逻辑层（核心逻辑在这里）
│   │   ├── platform/          # 第三方平台集成（OAuth）
│   │   ├── payment/           # 支付集成（Stripe、Airwallex、微信）
│   │   ├── pkg/               # 通用工具包
│   │   ├── server/            # HTTP 服务器配置和路由注册
│   │   ├── setup/             # 安装向导
│   │   ├── util/              # 内部工具函数
│   │   ├── web/               # 前端静态文件嵌入和 HTML 缓存
│   │   ├── integration/       # 外部服务集成
│   │   ├── securityaudit/     # 安全审计
│   │   └── testutil/          # 测试工具
│   ├── migrations/            # 数据库迁移文件
│   ├── pkg/                   # 对外暴露的工具包（pluginapi）
│   ├── resources/             # 嵌入资源
│   ├── dev-docs/              # 后端开发文档
│   ├── scripts/               # 脚本
│   ├── go.mod                 # Go 依赖（Go 1.27.3）
│   └── Dockerfile             # 后端构建镜像
├── frontend/                  # Vue 3 前端（TypeScript + TailwindCSS）
│   ├── src/
│   │   ├── api/               # API 调用封装（axios 封装，按模块分文件）
│   │   │   └── admin/         # 管理员 API
│   │   ├── assets/            # 静态资源（图片、图标）
│   │   │   └── icons/         # 图标
│   │   ├── components/        # 通用组件（按功能分子目录）
│   │   │   ├── common/        # 通用 UI 组件
│   │   │   ├── layout/        # 布局组件（Header、Sidebar）
│   │   │   ├── charts/        # 图表组件
│   │   │   ├── auth/          # 认证相关组件
│   │   │   ├── admin/         # 管理员组件
│   │   │   ├── keys/          # API Key 组件
│   │   │   ├── account/       # 账户组件
│   │   │   ├── channels/      # 渠道组件
│   │   │   ├── modelPlaza/    # 模型广场组件
│   │   │   ├── tickets/       # 工单组件
│   │   │   ├── payment/       # 支付组件
│   │   │   ├── user/          # 用户组件
│   │   │   └── Guide/         # 引导组件（driver.js）
│   │   ├── composables/       # Vue 组合式函数（useSeoMeta 等）
│   │   ├── constants/         # 常量定义
│   │   ├── features/          # 功能模块（相对独立的子功能）
│   │   │   ├── channel-monitor-v2/  # 渠道监控 v2
│   │   │   └── prompt-audit/  # Prompt 审计
│   │   ├── i18n/              # 国际化（en/zh）
│   │   │   └── locales/       # 翻译文件（en.ts、zh.ts）
│   │   ├── router/            # Vue Router 配置
│   │   ├── stores/            # Pinia 状态管理
│   │   ├── styles/            # 全局样式
│   │   ├── types/             # TypeScript 类型定义
│   │   ├── utils/             # 工具函数
│   │   ├── views/             # 页面组件
│   │   │   ├── admin/         # 管理员页面
│   │   │   ├── auth/          # 认证页面
│   │   │   ├── user/          # 用户页面
│   │   │   ├── public/        # 公开页面
│   │   │   └── setup/         # 安装向导页面
│   │   ├── App.vue            # 根组件
│   │   └── main.ts            # 入口文件（注册 pinia/router/i18n/head）
│   ├── public/                # 静态文件（robots.txt、sitemap.xml、llms.txt）
│   ├── package.json           # 前端依赖（用 pnpm）
│   ├── pnpm-lock.yaml         # pnpm 锁文件（必须提交）
│   ├── index.html             # HTML 入口（含 SEO meta）
│   └── vite.config.ts         # Vite 构建配置
├── deploy/                    # 部署配置（Docker Compose、K8s）
├── docs/                      # 用户文档
├── openspec/                  # OpenAPI 规范
└── tools/                     # 开发工具脚本
```

---

## 技术栈

### 后端

| 技术 | 版本 | 说明 |
|------|------|------|
| **Go** | 1.27.3 | 后端语言 |
| **Ent** | 0.14.5 | ORM 框架（Facebook 出品） |
| **Gin** | - | HTTP 框架 |
| **PostgreSQL** | 15+ | 主数据库 |
| **Redis** | 7+ | 缓存 + 会话 |

### 前端

| 技术 | 版本 | 说明 |
|------|------|------|
| **Vue** | 3.5+ | 前端框架（Composition API） |
| **Vue Router** | 4.2+ | 路由 |
| **Pinia** | 2.1+ | 状态管理 |
| **Vue I18n** | 9.14+ | 国际化（en/zh） |
| **TailwindCSS** | 3.4+ | 样式框架 |
| **Vite** | 5.0+ | 构建工具 |
| **TypeScript** | 5.6+ | 类型系统 |
| **@vueuse/core** | 10.7+ | Vue 组合式工具库 |
| **@vueuse/head** | 2.0+ | 动态 meta 标签（SEO） |

---

## 核心功能模块

### 后端（`backend/internal/`）

| 模块 | 路径 | 说明 |
|------|------|------|
| **用户管理** | `handler/user_handler.go`<br>`service/user.go`<br>`repository/user_repo.go` | 注册、登录、OAuth（GitHub/Google/微信/钉钉/OIDC）、邮箱验证 |
| **API Key** | `handler/apikey_handler.go`<br>`service/api_key.go` | API Key 生成、管理、配额 |
| **模型管理** | `handler/model_handler.go`<br>`service/model.go` | 支持 Claude、OpenAI、Gemini 等多个平台 |
| **渠道管理** | `handler/channel_handler.go`<br>`service/channel.go` | 上游渠道配置、监控、负载均衡 |
| **订阅计费** | `handler/subscription_handler.go`<br>`service/subscription.go` | 订阅套餐、充值、配额分发 |
| **支付集成** | `payment/stripe.go`<br>`payment/airwallex.go`<br>`payment/wechat.go` | Stripe、Airwallex、微信支付 |
| **使用统计** | `handler/usage_handler.go`<br>`service/usage.go` | 用户调用量、成本统计、渠道监控 |
| **管理后台** | `handler/admin/` | 用户管理、渠道配置、审计日志、风控 |
| **安全审计** | `securityaudit/` | Prompt 审计、内容审核 |
| **安装向导** | `setup/` | 首次部署时的配置向导 |

### 前端（`frontend/src/`）

| 模块 | 路径 | 说明 |
|------|------|------|
| **首页** | `views/HomeView.vue` | 公开落地页 |
| **认证** | `views/auth/` | 登录、注册、OAuth 回调、忘记密码 |
| **仪表盘** | `views/DashboardView.vue` | 用户概览 |
| **API Key** | `views/KeysView.vue` | Key 管理 |
| **使用统计** | `views/UsageView.vue` | 调用量、成本图表 |
| **订阅充值** | `views/PurchaseView.vue` | 购买套餐、充值 |
| **支付** | `views/payment/` | Stripe、Airwallex、微信、二维码 |
| **模型广场** | `views/ModelPlazaView.vue` | 可用模型展示 |
| **Playground** | `views/PlaygroundView.vue` | 在线调试 |
| **工单** | `views/TicketsView.vue` | 用户工单 |
| **管理后台** | `views/admin/` | 管理员功能（用户、渠道、订阅、审计） |
| **国际化** | `i18n/locales/en.ts`<br>`i18n/locales/zh.ts` | 中英文翻译 |
| **状态管理** | `stores/` | auth、app、admin、quota 等 |
| **SEO** | `composables/useSeoMeta.ts`<br>`public/sitemap.xml`<br>`public/llms.txt` | SEO 优化（2026-11-03 新增） |

---

## 开发工作流

### 0. 开发前必读

1. **先读这份文档（CLAUDE_DEV.md）**，了解项目结构。
2. **读 AGENTS.md**，知道不能编译、功能完成后要提交 git。
3. **优先复用代码**：搜索 `backend/internal/` 和 `frontend/src/` 看有没有现成的组件/函数/API。

### 1. 接到任务后

```bash
# 1. 搜索相关代码（找可复用的）
grep -r "关键词" backend/internal frontend/src

# 2. 读相关文件（不要只看函数名，要看实现）
Read backend/internal/service/xxx.go
Read frontend/src/views/xxx.vue

# 3. 开始写代码（优先复用现有函数、组件、样式）
```

### 2. 写代码时

- **后端**：
  - 新增 API：`handler/` 里加 handler，`service/` 加业务逻辑，`repository/` 加数据库操作。
  - 修改数据库：编辑 `ent/schema/`，跑 `go generate ./...` 生成代码（但 AGENTS.md 说不编译，所以只改 schema，告诉用户自己跑）。
  - 错误处理：用 `pkg/httputil` 的 `RespondError`，不要直接 `c.JSON`。
  - 日志：用 `log` 包（项目里有统一封装）。

- **前端**：
  - 新增页面：`views/` 加 `.vue`，`router/index.ts` 加路由，`i18n/locales/` 加翻译。
  - API 调用：`api/` 里封装，返回 Promise，用 axios。
  - 状态管理：优先在组件里用 `ref/reactive`，跨页面共享才用 Pinia store。
  - 样式：用 Tailwind class，不要写 `<style scoped>`（除非必须）。
  - 国际化：所有文案用 `$t('key')`，不要写死中文或英文。
  - SEO：新页面在 `router/index.ts` 的 `meta` 里加 `titleKey/descriptionKey`，然后在 `useSeoMeta.ts` 里调用。

### 3. 功能写完后

```bash
# 1. 静态检查（不编译）
git diff --check

# 2. 提交（只提交本次改动的文件）
git add backend/internal/handler/xxx.go frontend/src/views/xxx.vue
git commit -m "feat: 功能描述

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"

# 3. 更新这份文档（CLAUDE_DEV.md）
# 在"功能清单"章节加一行新功能
```

### 4. 功能清单更新规则

每完成一个功能，在下面"功能清单"章节加一行：

```markdown
| 2026-11-03 | SEO 优化 | 加 meta、sitemap、llms.txt、@vueuse/head | frontend/index.html, frontend/src/composables/useSeoMeta.ts | 085b9aad7 |
```

---

## 功能清单

| 日期 | 功能 | 说明 | 主要文件 | Commit |
|------|------|------|----------|--------|
| 2026-11-11 | 糖果推理与 SVG 智商检测合并 | 智商检测页新增糖果/SVG 切换；复用 API Key 分组、原生协议调用及定时调度，固定糖果题以独立整数 21 判分，区分未通过与检测失败；展示分组通过率、最近历史和纯文本回答，失败不计入通过率，糖果历史保留最近 100 次；新增迁移 264，旧 SVG 默认兼容；已补用例，语法、国际化与差异静态检查通过，未运行构建、测试或真实上游调用 | `backend/internal/service/intelligence_test_candy.go`<br>`backend/internal/service/intelligence_test_service.go`<br>`backend/migrations/264_intelligence_candy_test.sql`<br>`frontend/src/components/intelligenceTest/CandyTestResults.vue`<br>`frontend/src/views/admin/IntelligenceTestAdminView.vue` | 69c335402 |
| 2026-11-11 | 上游同步计划与余额邮件告警 | 每个上游可配置带时区的 Cron、余额阈值和收件邮箱；复用调度锁与 SMTP，数据库领取计划并去重告警，余额恢复后重置、失败可重试；新增迁移 263，仅静态检查，未构建/运行测试/真实发信 | `backend/internal/service/upstream_provider_automation.go`<br>`backend/internal/repository/upstream_provider_automation.go`<br>`backend/internal/service/upstream_provider_runner.go`<br>`frontend/src/components/admin/upstream/UpstreamProviderFormDialog.vue` | 0c0ea9d04 |
| 2026-11-11 | 邀请返现与 USDT 打款凭证 | 邀请返利统一更名返现，默认 6.8 人民币/USDT、后台及单笔可调整汇率；支持 Polygon/BSC，管理员上传截图和交易哈希后，在同一事务内创建凭证工单并扣减可提现金额，按网络与交易哈希防重复登记；用户可查历史凭证并继续工单沟通。复用工单图片存储，新增迁移 262；语法、国际化和差异静态检查通过，未运行构建与测试 | `backend/internal/service/affiliate_payout.go`<br>`backend/internal/repository/affiliate_repo.go`<br>`backend/migrations/262_affiliate_usdt_payout.sql`<br>`frontend/src/views/user/AffiliateView.vue`<br>`frontend/src/views/admin/affiliates/AffiliateOfflineWithdrawDialog.vue` | 0ee769ddb |
| 2026-11-11 | 上游建号本地分组多平台筛选 | 创建账号弹窗的本地分组支持多平台筛选和全部平台重置，保留跨平台已选分组并显示选中数量，重开弹窗重置筛选；复用平台目录与国际化文案 | `frontend/src/components/admin/upstream/UpstreamProvisionDialog.vue` | 5c7025547 |
| 2026-11-11 | 合并上游 main 到 new | 合入 up/main（530c5e3a4），解决 17 个文件冲突，保留工单、SVG 测试、供应商溯源及人民币展示；修复受限管理查询参数并升级认证缓存版本；静态检查、国际化检查及 dev Compose 镜像构建通过（构建时改用官方 Go 下载源） | `backend/cmd/server/wire_gen.go`<br>`backend/internal/repository/custom_admin_key_scope.go`<br>`backend/internal/service/antigravity_gateway_service.go`<br>`backend/internal/service/api_key_auth_cache_impl.go`<br>`frontend/src/views/admin/SettingsView.vue`<br>`frontend/src/views/user/KeysView.vue` | 1f5ed80e9 |
| 2026-11-10 | SVG 测试提示词模板复用 | 接入现有数据库提示词模板，支持命名保存、跨账号模型选择复用和历史提示词回填；模型选项转换为 SelectOption，修复 TS2322 | `frontend/src/components/admin/account/AccountIntelligenceTestPanel.vue`<br>`frontend/src/components/admin/account/__tests__/AccountIntelligenceTestPanel.spec.ts`<br>`frontend/src/i18n/locales/{zh,en}/admin/accounts.ts` | 8804ea134 |
| 2026-11-10 | 指定账号模型的 SVG 智商测试 | 账号测试弹窗支持选择或输入模型、自定义提示词及推理等级，复用 SVG 生成和安全预览；后台执行，按账号持久化提示词与 SVG 历史，支持预览、复制、下载和翻查，仅管理员可见；新增迁移 261 | `backend/internal/service/intelligence_test_account.go`<br>`backend/internal/repository/intelligence_test_account_repo.go`<br>`backend/migrations/261_account_intelligence_tests.sql`<br>`frontend/src/components/admin/account/AccountIntelligenceTestPanel.vue` | 791001b98 |
| 2026-11-09 | 上游 Key 新旧分组结构兼容用例 | 覆盖 group 缺失、null、字符串、对象及对象附带 groups 数组的创建响应，保留旧结构兼容 | `backend/internal/service/upstream_provider_client_test.go` | 5410de68c |
| 2026-11-09 | 上游创建账号分组解析修复 | 忽略 sub2api 创建 Key 响应中的 group 对象，保留 NewAPI 分组赋值，并补充分组对象响应回归用例 | `backend/internal/service/upstream_provider_client.go`<br>`backend/internal/service/upstream_provider_client_test.go` | 5300c767a |
| 2026-11-06 | 邀请返利推广文案一键复制 | 邀请页新增"推广文案"区块，i18n 模板自动拼站点名、邀请链接和邀请码，一键整段复制分享 | `frontend/src/views/user/AffiliateView.vue`<br>`frontend/src/i18n/locales/zh/dashboard.ts`<br>`frontend/src/i18n/locales/en/dashboard.ts` | 174f3fa64 |
| 2026-11-04 | 工单回复共享限流 | 用户新建工单和回复任意工单共享账号及 IP 每分钟一次的额度，补充新建与回复交叉限流场景测试 | `backend/internal/server/middleware/ticket_rate_limit.go`<br>`backend/internal/server/middleware/ticket_rate_limit_test.go`<br>`backend/internal/server/routes/user.go` | 74eac0bf8 |
| 2026-11-04 | 工单提交限流 | 新建工单按账号和安全客户端 IP 分别限制每分钟一次，任一超限返回 429 和 Retry-After；失败请求计数，Redis 故障时拒绝提交 | `backend/internal/server/middleware/ticket_rate_limit.go`<br>`backend/internal/server/middleware/ticket_rate_limit_test.go`<br>`backend/internal/server/routes/user.go` | 7ec03572f |
| 2026-11-03 | SEO + 国际化优化 | 加 meta description、Open Graph、JSON-LD、sitemap 多语言、llms.txt、@vueuse/head | `frontend/index.html`<br>`frontend/public/sitemap.xml`<br>`frontend/public/llms.txt`<br>`frontend/src/composables/useSeoMeta.ts`<br>`frontend/src/main.ts`<br>`frontend/src/router/title.ts` | 085b9aad7 |
| 2026-11-03 | 智能测试 SVG 提取修复 | `extractSVG` 增加字符串 fallback，解决模型返回非严格 XML（未转义 `&`、HTML 实体）时任务误判为 `svg_missing_or_invalid` 的问题，确保每次定时任务都能正确生成 SVG | `backend/internal/service/intelligence_test_service.go` | 6525e50b7 |
| 2026-11-04 | 账号指纹校对 | 用 LM Fingerpoint Detector 核对上游账号实际回的是不是声称的模型，识别中转商拿便宜模型冒充。按账号 opt-in，三个入口（接入时、每 6 小时抽查、手动触发）；判定四态，只有 mismatch 计入连续计数，连续 2 次才执行动账号的动作；集群级并发闸 + 每日配额 + 消耗记账 | `backend/migrations/260_fingerprint_plans.sql`<br>`backend/internal/service/fingerprint_*.go`<br>`backend/internal/repository/fingerprint_repo.go`<br>`backend/internal/handler/admin/fingerprint_handler.go`<br>`frontend/src/components/admin/account/FingerprintPanel.vue`<br>`frontend/src/api/admin/fingerprint.ts`<br>`frontend/src/views/admin/fingerprintBadge.ts` | (本次提交) |

---

## 常用 API 路由（后端）

| 路由 | 说明 | Handler |
|------|------|---------|
| `POST /api/auth/login` | 登录 | `handler/auth_handler.go` |
| `POST /api/auth/register` | 注册 | `handler/auth_handler.go` |
| `GET /api/user/me` | 当前用户信息 | `handler/user_handler.go` |
| `GET /api/keys` | 用户的 API Keys | `handler/apikey_handler.go` |
| `POST /api/keys` | 创建 API Key | `handler/apikey_handler.go` |
| `GET /api/models` | 可用模型列表 | `handler/model_handler.go` |
| `GET /api/usage` | 使用统计 | `handler/usage_handler.go` |
| `GET /api/subscriptions` | 用户订阅 | `handler/subscription_handler.go` |
| `POST /api/payment/stripe/checkout` | Stripe 支付 | `handler/payment_handler.go` |
| `GET /api/admin/users` | 管理员：用户列表 | `handler/admin/user_handler.go` |
| `GET /api/admin/channels` | 管理员：渠道管理 | `handler/admin/channel_handler.go` |

---

## 常用前端路由

| 路由 | 页面 | 组件 |
|------|------|------|
| `/` | 首页（重定向到 /dashboard 或 /home） | `views/HomeView.vue` |
| `/home` | 公开首页 | `views/HomeView.vue` |
| `/login` | 登录 | `views/auth/LoginView.vue` |
| `/register` | 注册 | `views/auth/RegisterView.vue` |
| `/dashboard` | 用户仪表盘 | `views/DashboardView.vue` |
| `/keys` | API Key 管理 | `views/KeysView.vue` |
| `/usage` | 使用统计 | `views/UsageView.vue` |
| `/purchase` | 订阅/充值 | `views/PurchaseView.vue` |
| `/model-plaza` | 模型广场 | `views/ModelPlazaView.vue` |
| `/playground` | Playground | `views/PlaygroundView.vue` |
| `/admin/dashboard` | 管理后台 | `views/admin/DashboardView.vue` |
| `/admin/users` | 用户管理 | `views/admin/UsersView.vue` |
| `/admin/channels` | 渠道管理 | `views/admin/ChannelsView.vue` |

---

## 数据库 Schema（Ent）

主要表（`backend/ent/schema/`）：

| Schema | 说明 |
|--------|------|
| `account.go` | 用户账户（余额、配额、订阅状态） |
| `account_group.go` | 用户组 |
| `api_key.go` | API Key |
| `auth_identity.go` | 第三方登录身份（GitHub/Google/微信等） |
| `announcement.go` | 公告 |
| `channel.go` | 上游渠道配置 |
| `channel_monitor.go` | 渠道监控 |
| `subscription.go` | 订阅套餐 |
| `order.go` | 订单 |
| `payment_record.go` | 支付记录 |
| `usage.go` | 使用记录 |
| `audit_log.go` | 审计日志 |
| `ticket.go` | 用户工单 |

---

## 可复用的工具和组件

### 后端工具（`backend/internal/pkg/`）

| 包 | 说明 |
|----|------|
| `httputil` | HTTP 响应封装（`Respond`、`RespondError`） |
| `jwtutil` | JWT 生成和验证 |
| `hashutil` | 密码哈希 |
| `validator` | 请求参数校验 |
| `ratelimit` | 限流 |
| `cache` | Redis 缓存封装 |

### 前端工具（`frontend/src/utils/`）

| 文件 | 说明 |
|------|------|
| `request.ts` | axios 封装（自动加 token、错误处理） |
| `format.ts` | 格式化（日期、金额、数字） |
| `validation.ts` | 表单校验 |
| `branding.ts` | 站点品牌（logo、名称） |
| `device.ts` | 设备检测（iOS/Android） |
| `siteBillingMode.ts` | 计费模式（充值/订阅） |

### 前端组件（`frontend/src/components/`）

| 组件 | 说明 |
|------|------|
| `Button.vue` | 按钮 |
| `Input.vue` | 输入框 |
| `Modal.vue` | 弹窗 |
| `Table.vue` | 表格 |
| `Loading.vue` | 加载动画 |
| `Toast.vue` | 消息提示 |
| `Pagination.vue` | 分页 |

---

## 测试

### 后端测试

```bash
# 单元测试
cd backend && go test -tags=unit ./...

# 集成测试
cd backend && go test -tags=integration ./...

# 代码质量检查
cd backend && golangci-lint run ./...
```

### 前端测试

```bash
cd frontend

# 运行测试
pnpm test

# 测试覆盖率
pnpm test:coverage

# 类型检查
pnpm typecheck

# Lint
pnpm lint
```

---

## 环境变量

### 后端（`backend/.env.example`）

| 变量 | 说明 |
|------|------|
| `DATABASE_URL` | PostgreSQL 连接串 |
| `REDIS_ADDR` | Redis 地址 |
| `JWT_SECRET` | JWT 密钥 |
| `STRIPE_SECRET_KEY` | Stripe 密钥 |
| `GITHUB_CLIENT_ID` | GitHub OAuth |
| `GOOGLE_CLIENT_ID` | Google OAuth |

### 前端（运行时从后端 `/api/settings/public` 拉取）

前端配置在运行时从后端获取，不需要环境变量。

---

## 依赖安装

### 后端

```bash
cd backend
go mod download
```

### 前端

```bash
cd frontend
pnpm install  # 必须用 pnpm，不要用 npm
```

---

## 常见问题

### 1. 前端依赖装不上

**原因**：用了 npm 而不是 pnpm。

**解决**：

```bash
cd frontend
rm -rf node_modules
pnpm install
```

### 2. CI 报 pnpm-lock.yaml 不一致

**原因**：修改了 `package.json` 但没提交 `pnpm-lock.yaml`。

**解决**：

```bash
cd frontend
pnpm install
git add pnpm-lock.yaml
git commit -m "chore: update pnpm-lock.yaml"
```

### 3. Go 版本不对

**原因**：CI 要求 Go 1.27.0，但本地或 CI 跑的是其他版本。

**解决**：检查 `backend/go.mod` 的 `go 1.27.x`，确保 CI workflows 里的版本断言一致。

### 4. 后端改了 schema 但代码没生成

**原因**：修改了 `ent/schema/` 但没跑 `go generate`。

**解决**：告诉用户自己跑 `cd backend && go generate ./...`（根据 AGENTS.md 规则，Claude 不编译）。

---

## 编码规范

### 通用

- **优先复用**：先搜索现有代码，不要重复造轮子。
- **命名**：清晰、语义化，不要缩写（除了约定俗成的如 `ID`、`URL`）。
- **注释**：只注释"为什么"，不注释"做什么"（代码本身应该足够清晰）。
- **错误处理**：不要忽略错误，用 `if err != nil` 或 `try-catch`。

### 后端（Go）

- 遵循 [Effective Go](https://golang.org/doc/effective_go)。
- 包命名：小写单数（`handler`、`service`、`repository`）。
- 导出函数首字母大写，内部函数小写。
- 用 `context.Context` 传递请求上下文。

### 前端（Vue 3 + TypeScript）

- 用 Composition API，不要 Options API。
- 组件文件名用 PascalCase（`HomeView.vue`）。
- Props 和 Emits 用 TypeScript 类型。
- 样式优先用 Tailwind class，少用 `<style scoped>`。
- 所有文案用 `$t('key')`，不要写死字符串。

---

## 部署

### 开发环境

```bash
# 后端
cd backend && go run cmd/server/main.go

# 前端
cd frontend && pnpm dev
```

### 生产环境

```bash
# Docker Compose
docker-compose -f deploy/docker-compose.yml up -d

# 或手动构建
cd backend && go build -o sub2api cmd/server/main.go
cd frontend && pnpm build
```

---

## 联系和贡献

- **GitHub**：https://github.com/Wei-Shaw/sub2api
- **文档贡献**：每次完成功能后，更新本文档的"功能清单"章节。

---

## 附录：目录树（精简版）

```
sub2api/
├── CLAUDE_DEV.md          # 本文档（开发前必读）
├── AGENTS.md              # Git 工作流规则
├── DEV_GUIDE.md           # 环境配置
├── README.md              # 项目介绍
├── backend/
│   ├── cmd/server/        # 主服务入口
│   ├── ent/schema/        # 数据库 schema
│   ├── internal/
│   │   ├── handler/       # HTTP handlers
│   │   ├── service/       # 业务逻辑
│   │   ├── repository/    # 数据库访问
│   │   ├── middleware/    # 中间件
│   │   └── pkg/           # 工具包
│   └── go.mod
├── frontend/
│   ├── src/
│   │   ├── api/           # API 调用
│   │   ├── components/    # 通用组件
│   │   ├── composables/   # 组合式函数
│   │   ├── i18n/          # 国际化
│   │   ├── router/        # 路由
│   │   ├── stores/        # 状态管理
│   │   ├── views/         # 页面
│   │   └── main.ts
│   ├── public/
│   │   ├── robots.txt
│   │   ├── sitemap.xml
│   │   └── llms.txt
│   ├── package.json
│   └── pnpm-lock.yaml
└── deploy/                # 部署配置
```

---

**最后更新**：2026-11-04
**维护者**：Claude Opus 5.5
