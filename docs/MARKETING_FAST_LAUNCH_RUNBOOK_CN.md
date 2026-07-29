# Tasksetly 快速推广执行手册

> 目的：在风口窗口内，用 72 小时完成最小可信上线，随后用 14 天验证一个主推工具和可复制渠道。
> 配套方案：[MARKETING_GROWTH_PLAN_CN.md](MARKETING_GROWTH_PLAN_CN.md)
> 技术规格：[MARKETING_TECH_SPEC_CN.md](MARKETING_TECH_SPEC_CN.md)
> 适用域名：`https://ai.tasksetly.com`

## 0. 执行规则

本手册只允许“受控快速推广”，不等同于立即购买大规模流量。

以下任一项未确认，停止公开推广：

- 上游明确允许当前地区、当前 API 转换方式和第三方收费调用。
- 对外宣传的模型、价格、试用额度和实际配置一致。
- 用户可以完成注册、创建 Key、真实调用和支付；支付失败或退款有负责人处理。
- 已有服务条款、隐私政策、使用限制、退款规则和客服入口。
- 试用额度可限制、可追踪、可停用；不能无限制赠送真实上游成本。

不要等待以下项目完成后才开始第 3 天推广：完整增长后台、完整 SEO 内容集群、自动联盟结算、D30/D90 cohort、所有工具的专属页面。

## 1. 角色和工作表

| 角色 | 责任 | 每日必须交付 |
| --- | --- | --- |
| 业务负责人 B | 价格、地区、授权、预算、宣传表述最终确认 | 决策记录和停损决定 |
| 工程负责人 E | 页面、公开接口、归因、修复和发布 | 发布记录、错误清单、验收结果 |
| 运营负责人 O | 内容、渠道、UTM、合作方和数据表 | 当日渠道数据和次日安排 |
| 客服负责人 S | 首调失败、退款、故障和用户反馈 | 工单分类和前三个阻塞原因 |

建立一个唯一的 `growth_daily` 表，字段固定为：

```text
日期、source、medium、campaign、content、落地页访问、CTA、注册、邮箱验证、创建 Key、首次真实调用、3 次调用、首付费、退款、试用成本、渠道成本、客服工单、首调失败数、备注
```

不要在第一个 72 小时内同时维护 GA4、Mixpanel、PostHog 等多个系统。先使用站内事件加表格。

## 2. T+0 至 T+6 小时：冻结销售方案

B 在 6 小时内写入并确认以下内容，不能使用“待定”：

```text
主推地区：
主推工具：
主推模型：
销售模式：余额 / 订阅 / 兑换码
公开价格：
计费单位：
试用额度：
试用有效期：
适用模型和限制：
退款条件：
客服入口和工作时间：
上游授权证据或合同位置：
允许公开使用的品牌和模型名称：
```

首期只选一个主推工具。优先选择当前真实可用性、用户需求和毛利同时最好的工具，例如 Codex；如果实际数据不支持，不得为了文案强行选择 Codex。

试用策略二选一：

- 已有 Turnstile、邮箱验证、注册频控和异常处置：开放小额自动试用。
- 以上条件未完成：关闭自动赠送，使用限量兑换码或人工审核邀请码。

## 3. T+6 至 T+24 小时：完成最小产品准备

### 3.1 后台配置

在现有管理设置中完成并截图留档：

- `contact_info`：客服入口、工作时间和首响承诺。
- `doc_url`：快速接入文档地址。
- `password_reset_enabled`：开启并测试邮件找回。
- 邮箱验证：至少测试一个新邮箱和一个过期链接。
- 支付开关：只打开已经实际配置并测试过的销售模式。
- 默认余额或订阅：与页面试用文案一致。
- Turnstile、注册频控和风险规则：试用开启时必须同时启用。
- 邀请返利：保持关闭，或只对指定合作方灰度，不公开全量开放。

### 3.2 页面最小内容

第一版只做一个主推落地页，必须有以下顺序：

1. 明确适用用户和主推工具。
2. 真实模型、价格、单位和更新时间。
3. 一段可复制配置，不包含真实 Key。
4. 从注册到首次成功调用的步骤。
5. 试用、退款、服务地区和客服规则。
6. 一个主 CTA：`复制配置并开始接入` 或 `查看价格`。

页面可以先使用人工维护的价格快照，但必须显示 `最后验证时间`。页面价格与后台配置不一致时，立即下线页面，不得继续投放。

### 3.3 当前代码需要的最小改动

当前前端是 Vue SPA，普通 Vue 路由的正文不会直接出现在服务器返回的 HTML 中。为了不等待 SSR，72 小时版本直接使用 Vite `public` 静态文件：

```text
frontend/public/go/codex          主推落地页，完整 HTML，无扩展名
frontend/public/robots.txt        抓取规则
frontend/public/sitemap.xml       只列当前真实公开页
frontend/public/og/codex.png      1200 x 630 分享图
```

`frontend/public/go/codex` 必须是完整 HTML，源代码内直接包含：

```html
<title>Codex API 接入与价格 | Tasksetly</title>
<meta name="description" content="按实际产品填写，不使用未经验证的稳定、官方或最低价承诺">
<link rel="canonical" href="https://ai.tasksetly.com/go/codex">
<meta property="og:title" content="Codex API 接入与价格 | Tasksetly">
<meta property="og:description" content="与页面可见内容一致">
<meta property="og:image" content="https://ai.tasksetly.com/og/codex.png">
<h1>Codex API 接入与价格</h1>
```

落地页使用内联 CSS 和最少量原生 JavaScript，不加载新的 UI 框架或分析 SDK。CTA 跳转到 `/register`，并把当前白名单 UTM 参数原样附加到注册链接。不要在 HTML 中写入任何真实 API Key。

生产环境需要不重新编译就更新价格时，在首次构建中保留同名静态占位文件，然后将实际文件部署到：

```text
data/public/go/codex
data/public/robots.txt
data/public/sitemap.xml
data/public/og/codex.png
```

现有前端服务会优先使用 `data/public` 中与嵌入静态文件同路径的文件。发布前必须确认实际部署工作目录中的 `data/public` 被持久化。

页面中的价格和模型首期允许来自一份负责人签字的快照；只允许在一个源文件维护。第 8-14 天再改为公共价格接口。

当前未知 SPA 路径会由后端返回 `index.html` 和 HTTP 200。该问题不阻塞第 3 天的社区首发，但必须在第 8-14 天修复；首发前至少确认所有已分发 campaign URL 都真实存在并返回正确页面。

## 4. T+24 至 T+48 小时：最小归因和发布

### 4.1 最小归因

第一版只记录以下字段：

```text
anonymous_id
first_source
first_medium
first_campaign
last_source
last_medium
last_campaign
landing_path
created_at
user_id（注册后绑定）
```

只接受白名单 UTM 字段：

```text
utm_source
utm_medium
utm_campaign
utm_content
utm_term
```

保留 30 天。不得记录完整 URL、邮箱、API Key、Authorization、提示词、响应正文或支付敏感 payload。

### 4.2 必须记录的服务端事实

第一版不需要完整事件平台，但以下事实必须可按用户和渠道回查：

```text
signup_completed
email_verified
api_key_created
first_api_success
third_api_success
payment_succeeded
refund_completed
```

注册、首次调用和支付至少能通过数据库查询导出到 `growth_daily`。支付成功只认 webhook 或订单最终状态，不能认前端支付成功页。

### 4.3 渠道链接

每个渠道单独生成链接，不允许复制同一个 URL：

```text
https://ai.tasksetly.com/go/codex?utm_source=linuxdo&utm_medium=community&utm_campaign=codex_fast_202607&utm_content=post_a
https://ai.tasksetly.com/go/codex?utm_source=bilibili&utm_medium=creator&utm_campaign=codex_fast_202607&utm_content=video_a
https://ai.tasksetly.com/go/codex?utm_source=github&utm_medium=opensource&utm_campaign=codex_fast_202607&utm_content=template_a
```

社区发布前由 O 在 `growth_daily` 登记 URL、负责人、发布时间和允许的宣传内容。

## 5. T+48 至 T+72 小时：发布前验收

### 5.1 工程命令

在仓库根目录执行：

```bash
make build
make test
```

如果完整测试超过发布窗口，至少执行：

```bash
make build-frontend
make test-backend
make test-frontend-critical
```

### 5.2 线上 HTTP 验收

```bash
BASE=https://ai.tasksetly.com

curl -fsS "$BASE/health"
curl -fsS -D /tmp/robots.headers "$BASE/robots.txt" -o /tmp/robots.txt
curl -fsS -D /tmp/sitemap.headers "$BASE/sitemap.xml" -o /tmp/sitemap.xml
curl -fsS -D /tmp/codex.headers "$BASE/go/codex" -o /tmp/codex.html
curl -fsS "$BASE/api/v1/settings/public" -o /tmp/public-settings.json
```

验收结果必须是：

```text
/health                         200
/robots.txt                     200 text/plain
/sitemap.xml                    200 application/xml 或 text/xml
/go/codex                       200 text/html，源代码包含页面 title、description、H1
/api/v1/settings/public          200，客服、文档和功能开关与首发配置一致
```

进一步检查页面源代码，不只检查浏览器渲染结果：

```bash
rg -n '<title>|description|canonical|og:title|og:description|Tasksetly|Codex' /tmp/codex.html
```

### 5.3 五组真实链路

使用 5 个新账号、至少 3 个网络执行：

```text
带 UTM 进入落地页
-> 查看价格
-> 注册
-> 邮箱验证
-> 创建 API Key
-> 复制配置
-> 在真实客户端调用成功
-> 连续成功 3 次
-> 创建订单
-> 支付成功
-> 检查服务端数据
```

每组必须记录：首次调用耗时、使用模型、错误信息、实际扣费、客服是否介入。任何一个账号出现真实 Key 泄露、扣费错误或无法退款，停止公开发布并修复。

## 6. 第 3-7 天：受控发布

第一天只发布三类内容，每类一条：

| 内容 | 具体主题 | CTA |
| --- | --- | --- |
| 接入教程 | 主推工具 3 分钟配置和验证 | 复制配置 |
| 故障排查 | 401、429、模型名和 Base URL 错误 | 查看排查步骤 |
| 成本说明 | 真实模型价格、计费单位和示例 | 查看价格 |

发布规则：

- 每个平台使用独立 UTM。
- 不在不同平台复制完全相同的硬广文案。
- 明确作者与 Tasksetly 的关系。
- 不使用“官方源、零封号、永不宕机、全网最低价”等未验证表述。
- 先发布解决问题的内容，再给产品链接。
- 每条内容发布后 2 小时内有人值守评论和首调问题。

合作方只签短周期测试，不先承诺永久返利：

```text
合作方数量：3-5 个
每个合作方唯一链接：是
基础制作费：预先固定
激活/首付费奖励：按服务端事实结算
注册奖励：否
测试周期：7 天
异常注册和退款：不计奖励
```

## 7. 第 8-14 天：渠道决策

每日 18:00 更新 `growth_daily`，按以下顺序决定动作：

| 条件 | 动作 |
| --- | --- |
| 产生注册但没有首次调用 | 暂停扩量，优先修复配置、模型名、额度和错误文案 |
| 首调成功但没有第 3 次调用 | 访谈用户，检查价格、稳定性和真实工作流价值 |
| 有激活但没有支付 | 检查价格、支付方式、充值路径和试用规则 |
| 有支付但退款或工单异常 | 立即暂停来源，先处理服务质量 |
| 连续 3 天产生激活和首付费 | 增加同类内容和同类合作方，不扩大到泛流量 |
| 只有点击和注册，没有激活 | 不追加预算，不把注册数当成功 |

初期渠道停损条件：

- 单渠道累计 50 个注册仍无一个真实激活。
- 首调失败率超过全站平均值的 1.5 倍。
- 出现 2 个以上相同支付或扣费投诉未在当日解决。
- 试用成本超过预设上限，或出现批量注册。
- 合作方隐瞒返利关系、误导宣传或无法解释流量来源。

## 8. 第 8-14 天并行工程任务

按优先级执行：

1. 将最小归因从表格迁移到 `growth_attributions` 和 `growth_events`。
2. 从真实用量日志派生首次、第 3 次和第 10 次成功调用。
3. 增加公开模型和价格接口，避免营销页面多处写死价格。
4. 增加公开状态页，并定义成功率、延迟和事件计算口径。
5. 为主推工具补充第二个错误排查页面。
6. 修复 SPA 软 404：已知前端路由继续返回 `index.html`，未知公共路径由后端返回 HTTP 404。
7. 将主推静态页迁移到可维护的预渲染页面；迁移前不得改变现有 campaign URL。
8. 只有以上数据稳定后，才灰度开启邀请返利。

## 9. 预算和发布节奏

没有可靠毛利数据前，所有广告和合作方都使用固定学习预算：

```text
社区内容：优先人工时间，不购买流量
创作者测试：每个合作方单独上限，先付制作费，不按注册结算
精确搜索：单关键词组 3 天测试，预算达到上限立即停
返利：首期关闭或只给审核合作方
泛信息流：14 天内不投
```

第一个付费广告测试必须满足：

- 已有至少 3 个自然来源首付费用户。
- 落地页到激活链路已通过 5 组全链路验收。
- 广告落地页与关键词一一对应。
- 预算由 B 预先写入表格，不能临时无限追加。

## 10. 14 天完成定义

14 天结束时必须能回答以下问题，并能从订单、用量和 `growth_daily` 回查：

1. 哪个渠道带来了首次真实调用？
2. 哪个渠道带来了第 3 次调用？
3. 哪个渠道带来了首付费？
4. 用户最常见的首调失败原因是什么？
5. 每个渠道消耗了多少试用成本和人工支持？
6. 哪个渠道应继续、暂停或改内容？

如果只有曝光、点击和注册，没有真实调用和支付数据，14 天推广结果记为“未验证”，不得扩大预算。
