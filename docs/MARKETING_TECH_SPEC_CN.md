# Tasksetly 推广技术与数据实施规格

> 版本：v1.0
> 制定日期：2026-07-22
> 对应计划：[MARKETING_GROWTH_PLAN_CN.md](MARKETING_GROWTH_PLAN_CN.md)

## 1. 目标与边界

本规格用于支持 Tasksetly 的 90 天推广计划，重点解决四件事：

1. 让未登录访客能看懂价格、模型、状态和接入方式。
2. 让公开页面可被搜索引擎正确抓取、分享和索引。
3. 把首次来源可靠关联到注册、首次成功调用和支付。
4. 在不记录 API Key、提示词或支付敏感信息的前提下计算漏斗、留存、CAC 和贡献毛利。

本规格不包含重做管理后台、替换支付系统、建设通用营销自动化平台或采集用户请求正文。

## 2. 当前可复用能力

现有代码已经具备以下能力，应优先复用：

- Vue/Vite 前端、Go 后端和 PostgreSQL 数据库。
- 邮箱注册与验证、API Key 创建、用量日志、支付订单和 webhook。
- Codex、Claude Code、Gemini CLI、OpenCode 等客户端配置片段。
- 可用渠道、模型定价、渠道监控和用户专属倍率。
- 邀请码、邀请链接、充值返利、冻结期、返利有效期、单被邀请人上限和返利转余额。
- 优惠码、兑换码、默认余额/订阅和 Turnstile 配置入口。

因此，邀请系统和客户端配置不需要从零开发。主要缺口是公开展示、来源归因、增长事件、激活任务和 SEO。

## 3. 建议工具

### 3.1 最小工具组合

| 目的 | 推荐工具 | 原因 | 是否需要开发 |
| --- | --- | --- | --- |
| 公共流量趋势 | Cloudflare Web Analytics | 当前站点已在 Cloudflare，低维护、无 Cookie 基础统计 | 主要为配置；需验证 CSP 和实际脚本 |
| 产品漏斗 | 站内一方 `growth_events` 事件链路 | 可与注册、API 用量、订单准确关联，不依赖跨境脚本 | 是 |
| 报表 | 第一阶段用 PostgreSQL 视图 + 管理员增长页；可选 Metabase | 避免一开始引入重型 CDP | 是；Metabase 可后置 |
| 搜索收录 | Google Search Console、Bing Webmaster Tools、百度搜索资源平台 | 查看抓取、关键词和索引问题 | 配置与站点验证 |
| 文档 | 复用 Vue 组件并生成静态页面，或使用 VitePress | 与现有技术栈一致，易维护代码示例 | 是 |
| 链接管理 | 统一 UTM 规则 + 受控表格/后台生成器 | 冷启动无需购买营销平台 | 先用表格；后续可开发 |
| 视频 | OBS/系统录屏 + 常用剪辑工具 | 真实操作比动画宣传更有说服力 | 否 |

不建议首期同时安装 GA4、PostHog、Mixpanel、Amplitude、Matomo 等多个分析系统。多个口径会增加重复事件和归因冲突。公共流量与业务结果两套数据足够完成前 90 天决策。

### 3.2 需要的人力

- 1 名熟悉现有 Go/Vue/PostgreSQL 的全栈工程师，P0 预计 16-27 人日。
- P1 预计另需 8-16 人日，可按实验结果分批做。
- 1 名业务负责人提供价格、毛利、试用、合规和文案决策。
- 1 名运营负责 UTM、内容、渠道和每周数据复盘。

## 4. 不开发即可完成的配置

以下事项应先通过现有后台或基础设施完成：

| 编号 | 配置 | 操作要点 | 验收 |
| --- | --- | --- | --- |
| C01 | 客服与文档 | 填写 `contact_info`、`doc_url` | 首页、登录页和应用内可找到有效入口 |
| C02 | 法律正文 | 补服务条款、隐私、使用政策、地区、退款和服务特定条款 | 未登录可访问，正文非空，有版本日期 |
| C03 | 账号找回 | 开启 `password_reset_enabled`，核对 `frontend_url` 和邮件 | 新用户能完成忘记密码全流程 |
| C04 | 试用 | 核对默认余额/订阅、有效期、模型范围 | 页面文案与测试账号实际权益一致 |
| C05 | 防注册滥用 | 配置 Turnstile、注册频控和风控处置流程 | 重复/异常注册被限制，正常注册可用 |
| C06 | 支付产品 | 明确余额充值和订阅购买开关、支付方式、价格 | 页面展示与实际下单结果一致 |
| C07 | 邀请返利灰度 | 配置比例、冻结期、有效期、单人上限；关闭管理员赠送返利 | 测试邀请、支付、冻结、退款、成熟和转余额 |
| C08 | 公共统计 | 开启 Cloudflare Web Analytics 或确认已有统计 | 无重复 pageview，隐私政策已说明 |
| C09 | 搜索平台 | 验证域名并提交 sitemap | 三个平台能读取 sitemap 和抓取状态 |

注意：仅打开 `risk_control_enabled` 不等于已经具备有效反作弊。必须同时有规则、告警、人工处置和申诉流程。

## 5. 开发任务与优先级

### 5.1 P0：放量前必须完成

| 编号 | 任务 | 估算 | 验收标准 |
| --- | --- | ---: | --- |
| D01 | SEO 与公开路由基础 | 3-5 人日 | 正确 title/description/canonical/OG；robots 为文本、sitemap 为 XML；未知公共路径返回真实 404 |
| D02 | 公共价格、模型与状态页 | 4-6 人日 | 未登录可访问；字段有更新时间；不暴露账号、代理、成本和内部 ID |
| D03 | 快速接入文档 | 2-4 人日 | Codex/Claude/Gemini 各由新测试账号照文档完成调用；代码可复制且不含真实 Key |
| D04 | 一方归因与核心增长事件 | 4-7 人日 | UTM 从首次访问关联到用户、首次成功调用和支付；刷新/重试不重复计数 |
| D05 | 新用户激活任务流 | 3-5 人日 | 用户能按工具完成验证、Key、配置、测试和真实调用；记录各步骤事件 |

P0 总计约 16-27 人日。D02、D03 可与 D04 并行，但同一工程师执行时按顺序交付。

### 5.2 P1：种子用户验证后完成

| 编号 | 任务 | 估算 | 验收标准 |
| --- | --- | ---: | --- |
| D06 | 增长报表与 cohort | 2-4 人日 | 可按来源、活动、落地页、工具查看漏斗、D1/D7、支付和毛利 |
| D07 | 邀请反作弊与渠道管理 | 2-4 人日 | 返利以服务端订单为准；退款回滚；可冻结来源；异常账户可追踪 |
| D08 | 内容/活动链接生成器 | 1-2 人日 | 按统一命名生成 UTM 和短链接，避免重复 campaign 名 |
| D09 | 团队线索入口 | 2-4 人日 | 企业/团队需求可提交规模、工具、预算和联系方式，并进入工单或 CRM |
| D10 | 内容更新时间提醒 | 1-2 人日 | 价格、模型或配置变更时能定位需更新的公开内容 |

## 6. SEO 和公开页面规格

### 6.1 推荐路由

```text
/
/pricing
/models
/status
/docs/quickstart
/guides/codex-cli
/guides/claude-code
/guides/gemini-cli
/faq/billing
/faq/errors
/legal/terms
/legal/privacy
/legal/refund
```

`/dashboard`、`/keys`、`/usage`、`/orders`、`/profile` 和 `/admin/*` 不进入 sitemap。robots 不是安全控制，受保护路由仍必须依靠鉴权。

### 6.2 渲染方案

优先级如下：

1. 对营销、价格、模型、状态和文档页面做静态生成或服务端预渲染。
2. 保留现有 SPA 处理登录后应用。
3. 如果使用 VitePress，可由同域 `/docs/` 提供文档，避免 iframe，并统一 Canonical 与统计域。
4. 不使用 `home_content` URL iframe 作为长期营销站方案；iframe 不利于搜索、归因、可访问性和页面性能。

如果首期工期紧，可先为少量固定公共路由生成静态 HTML，再逐步扩展内容系统，不需要一次性重构整个应用为 SSR。

### 6.3 每页必备元信息

- 唯一且具体的 `<title>`，中文页面避免只写 `AI API Gateway`。
- 120-160 字左右的真实页面描述。
- 绝对地址 Canonical。
- `og:title`、`og:description`、`og:url`、`og:image`、`og:type`。
- Twitter Card 元信息。
- 中文/英文都存在时提供正确 `hreflang`。
- 可见 H1 与 title 语义一致。
- `SoftwareApplication`、`FAQPage` 等结构化数据只在页面有对应可见内容时使用。

### 6.4 robots、sitemap 和 404 验收

```text
GET /robots.txt
Status: 200
Content-Type: text/plain

GET /sitemap.xml
Status: 200
Content-Type: application/xml 或 text/xml

GET /definitely-not-a-real-public-page
Status: 404
```

sitemap 只列可索引的规范 URL，包含最后更新时间。测试环境、登录后页面、参数页和重复语言页不得进入。

### 6.5 公共数据脱敏

公共状态和模型接口允许展示：

- 对外渠道名称或产品分组名称。
- 支持的模型、协议和工具。
- 对用户生效的公开价格/倍率与更新时间。
- 聚合成功率、P50/P95 延迟、最近公开事件。

禁止展示：

- 上游账号邮箱、Token、Cookie、API Key、代理地址和内部备注。
- 单账号配额、成本价、调度权重、内部 ID 和可推断账号池规模的数据。
- 单个用户请求、提示词、IP 或可识别使用模式。

## 7. 归因设计

### 7.1 UTM 命名

统一使用小写 ASCII `snake_case`，不含中文、空格、邮箱或用户 ID。

```text
utm_source=linuxdo
utm_medium=community
utm_campaign=codex_quickstart_202608
utm_content=tutorial_a
utm_term=codex_api
```

字段规则：

- `utm_source`：具体平台或合作方，如 `linuxdo`、`v2ex`、`bilibili`、`partner_x`。
- `utm_medium`：`organic`、`community`、`creator`、`affiliate`、`paid_search`、`email`。
- `utm_campaign`：主题 + 月份，如 `claude_guide_202608`。
- `utm_content`：帖子、素材或版本，如 `video_a`、`post_b`。
- `utm_term`：仅用于搜索关键词或明确内容标签。

### 7.2 首次与末次归因

同时保存两套口径：

- First touch：30 天内首次非直接来源，用于判断品牌发现渠道。
- Last non-direct：转化前最近一次非直接来源，用于判断促成渠道。

直接访问不能覆盖已经存在且未过期的非直接来源。内部页面跳转不能成为新来源。

### 7.3 匿名访问到用户

推荐流程：

1. 首次访问生成随机 `anonymous_id`，使用 `Secure`、`SameSite=Lax` 的一方 Cookie。
2. 保存 landing path、referrer host、UTM、首次时间和最近时间，默认保留 30 天。
3. 注册成功后由服务端将 `anonymous_id` 与 `user_id` 绑定。
4. 邮箱验证、Key 创建、API 调用和支付由服务端通过 `user_id` 关联。
5. 如果文档放在子域，需统一一方域策略或通过明确的跨域链接参数传递非敏感 attribution token。

联盟 `aff` 归因必须与普通 UTM 分开保存。佣金归属以服务端校验的邀请关系为准，不信任前端任意提交的合作方 ID。

## 8. 事件模型

### 8.1 建议公共字段

```text
event_id           UUID，幂等键
event_name         受控事件名
occurred_at        服务端时间或校正后的客户端时间
anonymous_id       匿名随机 ID，可空
user_id            用户 ID，可空
session_id         会话随机 ID，可空
path               不含敏感查询参数的路径
referrer_host      只存 host，避免保存完整敏感 URL
utm_source
utm_medium
utm_campaign
utm_content
utm_term
first_touch_id     首次来源记录 ID
last_touch_id      最近非直接来源记录 ID
properties         受白名单约束的 JSONB
```

禁止进入事件系统的数据：

- API Key、Authorization、Cookie、密码、验证码。
- 完整提示词、模型响应、上传文件或图片内容。
- 支付卡、钱包密钥、支付 provider 的敏感 payload。
- 完整 IP；安全系统确有需要时应与增长事件分表、限权并设置保留期。

### 8.2 核心事件目录

| 事件 | 产生端 | 去重/定义 | 主要属性 |
| --- | --- | --- | --- |
| `landing_viewed` | 前端/边缘 | 每会话每 landing 一次 | page_type、tool |
| `primary_cta_clicked` | 前端 | 每次点击 | cta_id、destination |
| `pricing_viewed` | 前端 | 每会话一次 | plan_type、tool |
| `docs_viewed` | 前端 | 每会话每文档一次 | doc_id、tool |
| `signup_started` | 前端 | 首次聚焦/提交之一，口径固定 | method |
| `signup_completed` | 后端 | 用户创建事务成功一次 | method、promo_present、aff_present |
| `email_verified` | 后端 | 首次验证成功一次 | elapsed_seconds |
| `api_key_created` | 后端 | Key 创建事务成功 | platform、group_id；不存 Key |
| `quickstart_copied` | 前端 | 每次明确复制 | tool、os、snippet_id |
| `test_request_succeeded` | 后端 | 站内测试工具成功 | platform、model、error_category |
| `first_api_success` | 后端 | 首次真实外部客户端成功且非站内测试 | platform、model、elapsed_from_signup |
| `third_api_success` | 后端派生 | 第 3 次真实成功调用一次 | platform、model |
| `tenth_api_success` | 后端派生 | 第 10 次真实成功调用一次 | platform、model |
| `checkout_started` | 后端优先 | 创建待支付订单 | product_type、amount_bucket、currency |
| `payment_succeeded` | 后端 webhook/订单 | provider transaction 幂等一次 | product_type、net_amount、currency |
| `refund_completed` | 后端 | 退款最终状态一次 | amount_bucket、reason_category |
| `affiliate_landing` | 前端 + 服务端 | 有有效 `aff` 的 landing | affiliate_code_hash、campaign |
| `affiliate_attributed` | 后端 | 邀请关系建立一次 | affiliate_user_id |
| `affiliate_rebate_matured` | 后端 | 冻结结束并有效入账一次 | amount、source_order_id |
| `ticket_created` | 后端 | 工单创建成功 | category、activation_stage |

金额用于报表时优先存订单表引用或精度明确的最小货币单位。前端事件不能作为支付、佣金或收入事实来源。

### 8.3 激活定义

激活由服务端派生，不由前端按钮点击决定：

```text
activated_at = 第 3 次满足以下条件的调用时间：
- 来源不是管理员“测试账号/测试渠道”功能
- 使用用户创建的真实 API Key
- 网关已返回业务成功
- 不是重复重试产生的同一幂等请求
- 注册时间后 24 小时内完成
```

后台健康检查、渠道测试、文档演示共享 Key 和管理员代调用都不得计入用户激活。

## 9. 数据表和服务设计建议

最低需要以下逻辑实体，可按现有仓库约定命名：

- `growth_attributions`：匿名 ID 的 first/last touch 和 landing 信息。
- `growth_events`：经过白名单校验的不可变事件。
- `growth_user_links`：匿名 ID 与用户的绑定记录。
- `growth_campaigns`：UTM 标准名、负责人、成本和状态。
- 数据库视图：`growth_funnel_daily`、`growth_cohort_weekly`、`growth_channel_economics`。

实现要求：

- 高价值后端事件与业务事务在同一事务提交，或通过可靠 outbox 异步写入。
- `event_id`、业务对象 ID + 事件名建立唯一约束，防 webhook 重试重复。
- 客户端事件入口有事件名和属性白名单、体积限制、频控和 CSRF/来源校验。
- 埋点失败不能阻塞注册、API 调用和支付主流程。
- API 高并发调用不应逐次同步写增长事件；首次/第 3/第 10 次事件可从现有用量日志异步派生。
- 所有时间统一以服务端 UTC 存储，报表按站点时区展示。

## 10. 增长报表

### 10.1 获客面板

- 有效访问、CTA、注册、激活、首付费，按 source/medium/campaign/content 展示。
- 每个落地页的访问到激活率。
- 首次来源和末次非直接来源两套结果。
- 自然、社区、联盟、付费和直接流量占比。

### 10.2 激活面板

- 注册到邮箱验证、Key 创建、配置复制、首次调用和第 3 次调用的转化与耗时。
- 按 Codex、Claude Code、Gemini CLI 等工具拆分。
- 首次调用失败的错误类别，不展示请求正文。
- P50/P90 首次成功调用耗时。

### 10.3 收入与留存面板

- 激活到首付费率、首付费耗时、二次付费率。
- D1/D7/D30 调用留存。
- 净收入、上游成本、支付费、退款、返利和贡献毛利。
- 每渠道付费 CAC、激活 CAC 和 90 天贡献毛利。

### 10.4 可靠性与风险面板

- 按平台/模型的成功率、错误类别、P50/P95 延迟。
- 试用消耗、异常注册、共享设备/网络信号和处置结果。
- 返利冻结、退款回滚、异常合作方和拒付。
- 每 100 个激活用户产生的工单量。

## 11. 新用户激活任务流

推荐登录后的顺序：

1. 验证邮箱。
2. 选择要接入的工具：Codex、Claude Code、Gemini CLI 或其他已支持工具。
3. 显示该工具可用模型和实际价格，选择分组。
4. 创建 API Key。
5. 按操作系统显示一段可复制配置，敏感 Key 默认遮罩。
6. 运行站内安全测试，明确它不计入真实激活。
7. 引导用户回到真实客户端完成调用，后台检测成功后自动完成任务。
8. 根据实际产品显示充值、订阅或继续试用，不混淆两种计费模式。

任务流应支持跳过、稍后继续和重新打开。不能只依赖一次性浮层教程；用户真正关心的是当前还缺哪一步以及失败如何修复。

每个错误状态至少给出：错误类别、可执行修复、相关文档和客服入口。不得在浏览器控制台或分析事件中打印完整 Key。

## 12. 邀请返利技术护栏

- 邀请关系在注册成功时由服务端固定，普通用户不能自行更换邀请人。
- 自邀、循环邀请、批量相似账号和已存在用户补绑需要明确规则。
- 返利只基于最终成功且未退款的有效订单。
- 管理员赠送、兑换码、免费试用、测试订单默认不产生返利。
- 返利冻结期间订单退款或拒付应自动取消；成熟后退款要有冲正记录。
- 所有返利计算记录规则版本、比例、基数、订单和邀请双方。
- 停用合作方不删除历史账本；只阻止新归因或新返利。
- 合作方看到聚合结果，不看到被邀请人的敏感支付或使用明细。

上线前必须覆盖：正常邀请、重复点击、跨设备、先注册后点击、自邀、退款、重复 webhook、管理员充值、冻结成熟和上限封顶测试。

## 13. 测试与验收

### 13.1 自动化测试

- UTM 解析、清洗、过期、first touch 和 last non-direct 单元测试。
- 匿名 ID 绑定、重复注册和跨会话测试。
- 事件白名单、属性清洗、频控和幂等测试。
- 首次/第 3/第 10 次真实调用派生测试。
- 支付 webhook 重试、退款和返利冲正集成测试。
- robots、sitemap、元信息、Canonical 和 404 路由测试。
- 公共价格/状态接口的字段白名单测试。

### 13.2 全链路验收场景

使用至少 5 组全新测试账号，分别从不同 UTM、设备和网络执行：

```text
落地页 -> 价格/文档 -> 注册 -> 邮箱验证 -> 创建 Key
-> 复制配置 -> 站内测试 -> 真实客户端 3 次成功调用
-> 创建订单 -> 支付成功 -> 次日再次调用
```

逐项核对：

- 页面与后端事件数量一致且不重复。
- first touch、last non-direct、affiliate 三种归因符合预期。
- 报表中的用户、订单和贡献毛利可回查到业务事实。
- 日志、事件和页面源代码中没有 API Key、密码或支付敏感信息。
- 关闭分析服务或模拟写入失败时，核心产品仍可正常使用。

### 13.3 性能要求

- 埋点脚本不得阻塞首屏和主要 CTA。
- 客户端事件批量/异步发送，页面离开时允许丢失低价值行为事件。
- 服务端高价值事件必须可靠，但不能增加网关请求同步延迟。
- 公共价格和模型页使用短时缓存；状态页明确数据更新时间。

## 14. 发布顺序

### Release 1：可测量

- 配置客服、法律、密码重置和试用规则。
- 上线 UTM 保存、注册/Key/首调/支付后端事件。
- 建最小漏斗查询并用测试账号验证。

### Release 2：可发现

- 修复 robots、sitemap、Canonical、OG 和 404。
- 上线公共价格、模型、状态与三份快速接入文档。
- 提交搜索平台并检查抓取。

### Release 3：可激活

- 上线工具选择和新用户任务流。
- 根据种子用户数据修复首调失败。
- 增加 cohort、留存和毛利面板。

### Release 4：可扩张

- 灰度邀请返利和合作方渠道管理。
- 上线反作弊、退款冲正和渠道停损。
- 达到两个连续周 cohort 的质量与毛利门槛后再增加预算。

## 15. 完成定义

技术基础建设完成，不是指“统计脚本已加载”，而是负责人能够在同一周报中可靠回答：

1. 哪个来源、帖子或合作方带来了用户？
2. 用户在哪一步没有完成首次真实调用？
3. 哪些模型或错误影响激活与留存？
4. 哪个渠道带来的用户完成了付费和二次使用？
5. 扣除上游成本、支付费、退款和返利后，该渠道是否产生正贡献毛利？

只有五个问题都能用可回查的数据回答，推广技术工具才算真正交付。
