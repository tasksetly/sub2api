# 账号指纹校对 — 设计文档

日期：2026-11-04
状态：已与需求方确认，待实现

---

## 1. 意图与动机

管理员从中转商手里采购上游账号，接进账号池。中转商声称提供某个高价模型（例如 `claude-opus-5`）并给出低倍率，但实际上在后端把请求映射到一个便宜得多的模型（例如 `deepseek-v4-flash` 或同家族的 `claude-haiku-5.5`）。管理员按高价模型的预期付费和定价，用户拿到的却是低价模型的输出——钱亏了，口碑也亏了。

这种造假目前只能靠人工察觉"回答质量不对劲"，发现得晚，而且拿不出证据。

本功能引入 [LM Fingerpoint Detector](https://lm.ikale.io/docs/sdk/)：向账号发送整数生成挑战，用模型输出的数字分布特征反推背后真实模型，给出一个可追溯、可重复、可存档的判定结果。

**为什么现在做**：采购的上游账号数量在增长，人工抽查覆盖不过来；而且中转商的典型手法是前期给真货、等用量上来再偷偷换模型，一次性验收根本拦不住。

---

## 2. 目标

- 对**指定的**账号，用指纹检测核对它实际回的是不是声称的模型
- 三个触发入口都要有：
  1. **接入时校验** — 新建账号流程里勾选，建完立刻跑一次
  2. **定期抽查** — cron 调度，持续监控中转商中途换模型
  3. **手动触发** — 账号操作菜单里随时跑一次
- 判定不符时按计划配置执行动作，默认只告警不停号
- 保留完整检验结果（候选模型排名、置信度、原始判定），让管理员自己看证据、自己拍板

## 3. 非目标

- 不做全账号默认开启。**按账号 opt-in**，不建计划的账号完全不受影响，不产生任何额外请求
- 不做自动封禁。默认动作是标记 + 告警，停号与否由管理员决定（但可以配置成自动停用）
- 不做分词器探测、不做多轮累积置信度建模（SDK 本身也不提供）
- 不覆盖 OAuth 类账号（见「约束」）
- 不做跨账号的横向比对或供应商画像

## 4. 反目标（即使功能跑通也算失败的情况）

- **误伤好账号**：一次误判就把正常账号停掉，比漏检更糟。默认行为必须保守
- **烧钱**：指纹检测的输出 token 成本不可忽视，如果默认配置能让几百个账号同时开炮，这个功能就是负资产
- **只给结论不给证据**：管理员拿着"疑似造假"四个字没法去找中转商对质。必须能看到判成了什么模型、概率多少、几次了
- **沉默失败**：SDK 参考数据下载不了、账号类型不支持、检测请求失败——这些都必须在界面上说清楚，不能表现成"检测通过"

---

## 5. 约束

### 5.1 第三方依赖（已获需求方批准）

引入 LM Fingerpoint Detector 的 Go SDK（Go 1.23+），这是后端的第一个此类新依赖。项目规则默认禁止新增第三方依赖，本次已明确批准。

**模块路径（2026-11-04 核实）**：`github.com/Ikaleio/lm-detector/sdk/go`。Go SDK 是仓库里的子模块，**不是** `github.com/Ikaleio/lm-detector`。仓库为 MIT 协议。

SDK 自身只依赖 `github.com/klauspost/compress v1.18.4`（zstd 解压）。项目 `backend/go.mod:31` 已有 `v1.18.2`，属小版本上浮，不引入额外传递依赖。

### 5.2 参考数据在运行时下载

SDK 安装包不含参考数据。首次加载要拉约 3.6 MB 压缩数据（解压约 4.5 MB），默认源按顺序尝试：

1. `https://lm.ikale.io/data/`
2. `https://cdn.jsdmirror.com/npm/lmfpd-data@latest/`（国内镜像）

`loadDetector` 支持 `source` / `cache` / `ttl` 三个参数；`source` 传一个非 http(s) 的路径时当本地目录读取，完全离线。

**需求方决定：数据源做成系统设置项**，默认官方源 + 国内镜像兜底，内网部署可改为本地目录或自建镜像。缓存路径同样可配置。

加载开销 31ms、单次三条评分 2.3ms（Go），**detector 实例必须常驻复用，不能每次检测重新加载**。

### 5.3 只覆盖 baseUrl + apiKey 型账号（需求方明确选择）

SDK 的 `detect()` 自己构造 HTTP 请求，需要 `base_url` + `api_key` + `api` 三选一（`responses` / `chat_completions` / `messages`）。

曾提出备选方案 B：只用 SDK 的 `generateChallenges()` / `parseNumbers()` / `analyze()` 零件，挑战走项目现有的 `RunTestBackgroundWithOptions` 通道发送，这样 OAuth 账号、代理、平台差异全部复用现有逻辑，可以覆盖全平台。

**需求方选择方案 A（直接用 `detect()`）**，理由是被骗的场景基本都是 `upstream` / `apikey` 型中转商账号，A 足够覆盖且实现简单。

**代价（需求方已知悉并接受）**：OAuth / setup-token / bedrock / service_account 型账号无法使用本功能。界面上这些账号的入口要置灰并写明原因，不能让人误以为是功能故障。

#### 5.3.1 代理：已知局限（2026-11-04 核实源码后确认）

Go SDK 的两个 HTTP 客户端都是**包级私有变量，不可注入**：

```go
// sdk/go/sample.go:21  — 发检测请求
var sampleClient = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}
// sdk/go/data.go:170   — 下载参考数据
var dataClient = &http.Client{}
```

SDK 源码注释写明：`The default transport honors HTTPS_PROXY, HTTP_PROXY, and NO_PROXY`。即**只支持进程级环境变量代理，不支持按账号配置代理**。而账号表的 `proxy_id` 是一等公民，部分账号配有独立代理。

**这个局限的后果不只是"可能连不上"**：检测请求走直连、业务请求走代理时，上游看到的是两个不同来源 IP，理论上可以按 IP 区别对待（给检测流量发真货、给业务流量发假货）。SDK 文档强调要「固定请求参数」才能比较，而出口 IP 不一致意味着检测链路与业务链路不同，**结论不能无条件外推到真实业务**。

曾据此建议改用方案 B（检测请求走项目现有发送通道，天然共用代理与凭证）。**需求方在知悉以上后果后，决定维持方案 A。**

实现要求：
- 不得为了让检测跑通而静默绕过账号已配置的代理
- 配有代理的账号，其指纹检测结果在界面上必须标注"检测未经该账号代理，结论仅供参考"一类的说明
- 不要把这个局限藏在文档里——管理员看结果时就要看见

### 5.4 判定结果的性质

SDK 文档明确说明：排名是**参考库内的封闭集合排序**。库里没有的模型也会被硬配一个"最像"的候选，`probability` 只在库内模型之间有意义。文档建议「固定请求参数、重复多轮、结合其他证据」。

因此判定结果在产品语义上是**警告与证据**，不是**判决**。界面措辞和默认动作都要体现这一点。

截至 2026-11-03 的参考库有 66 个模型、16 个家族（Claude 12 / GPT 10 / Gemini 8 / DeepSeek 7 / GLM 4 / Qwen 4 / MiMo 4 / Kimi 3 / Grok 3 / Seed 3 / Muse 3 / openPangu 2 / Hunyuan 1 / MiniMax 1 / Step 1）。

### 5.5 检测成本

一次检测最多 3 个并发请求，每个要求模型输出 292–332 个整数，约 700–1000 输出 token，**单次检测约 2000–3000 输出 token**。默认配置必须以不烧钱为前提（见 6.4）。

### 5.6 项目既有规则

- 优先复用项目内已有实现，不重复造轮子
- 前端不新增任何第三方依赖
- 不运行编译/构建/测试命令，除非明确要求
- 功能完成后提交本地中文 commit，不 push
- 在 `CLAUDE_DEV.md` 的「功能清单」加一行记录

---

## 6. 已确定的设计决策

### 6.1 复用既有的「定时测试」子系统，不另起炉灶

项目里的定时测试子系统结构上几乎就是为这个功能长的：

| 既有能力 | 文件 |
|---|---|
| 计划表（cron / enabled / model_id / max_results） | `backend/migrations/066_add_scheduled_test_tables.sql` |
| 失败动作（停用 / 临时不可调度 / 移出分组） | `backend/migrations/253_scheduled_test_prompt_actions.sql` |
| 成功动作（启用 / 恢复 / 加入分组） | 同上 |
| 领域模型与仓储接口 | `backend/internal/service/scheduled_test_port.go` |
| 调度循环、计划锁、手动触发 `StartPlanNow` | `backend/internal/service/scheduled_test_runner_service.go` |
| CRUD service | `backend/internal/service/scheduled_test_service.go` |
| handler 与路由 | `backend/internal/handler/admin/scheduled_test_handler.go`、`backend/internal/server/routes/admin.go:784-799` |
| 前端面板 | `frontend/src/components/admin/account/ScheduledTestsPanel.vue` |
| 前端 API | `frontend/src/api/admin/scheduledTests.ts` |
| 账号操作菜单入口 | `frontend/src/components/admin/account/AccountActionMenu.vue` |

**决策（设计者判断，需求方已认可整体方向）**：给测试计划增加一个**校验类型（check type）**维度——现有的「发提示词对答案」保持为默认类型，新增「指纹校对」类型。cron 调度、计划锁、结果留存、失败/成功动作、手动触发全部原样复用。

实现者可以自行判断是复用同一张 `scheduled_test_plans` 表加类型字段，还是另建一组表共用 runner。前者改动小但会让表的字段语义变杂（prompt/expected_answer 对指纹类型无意义）；后者干净但要复制调度逻辑。**这是留给实现者的判断**，判据是：不要为了复用把两种类型的字段揉在一起到难以理解的程度，也不要把 runner 的调度/锁/手动触发逻辑复制一份。

### 6.2 三个入口

- **接入时校验**：新建账号表单里加一个勾选项「创建后立即做指纹校对」，**默认不勾**。

  具体落法（设计者判断）：勾选后**创建一条正常的指纹计划**（`enabled = true`，cron 取默认的每 6 小时），并立即触发一次检测——而不是做成"一次性检测"。理由有二：`scheduled_test_results.plan_id` 是 NOT NULL 外键，一次性检测没有计划可挂；而且接入时验过的账号本来就该继续被抽查，建计划让后续监控自然接上。

  勾选框展开后要同时提供**预期模型下拉**，否则无从判定。

  结果回传：创建弹窗在后端返回时就关闭了，而检测要几十秒，**不要让弹窗等检测**。检测异步跑，结果通过账号列表上的指纹标记 + 一条 toast 呈现。

  **批量创建与导入流程不提供这个勾选**（需求方拍板）。一次导入几十个账号全部开炮正是反目标「烧钱」描述的场景；导入后由管理员自己挑账号建计划。
- **定期抽查**：复用现有 cron 调度器与 `ListDue` 查询
- **手动触发**：复用 `StartPlanNow`，入口加在 `AccountActionMenu.vue`

### 6.3 判定规则（需求方拍板）

**一个计划只测一个模型**（需求方明确）：账号下即使挂了多个模型，指纹计划也只针对管理员指定的那一个跑，**不遍历账号的全部模型**。这既是成本控制，也让判定口径单一清晰。要测多个模型就建多个计划。

计划里有两个模型相关字段，不要混淆：

| 字段 | 含义 |
|---|---|
| **测试模型**（沿用既有 `model_id`） | 发给账号的请求里用的模型名，即上游声称提供的那个 |
| **预期模型**（新增） | 参考库里的模型 ID，用于和 `analysis.prediction` 比对 |

两者通常指同一个模型，但命名空间不同——上游可能用自编名字（`claude-opus-5-latest`、`opus-max`），参考库用规范化 ID（`claude-opus-5`）。

**判定：`analysis.prediction` ≠ 预期模型 → 判为不符，触发告警。**

需求方明确表示不分跨家族/同家族两档严厉程度——只要不是指定的那个型号，就按不符处理。

同时必须保留完整检验结果供人工复核：`decision`、`prediction` / `prediction_name`、`family_prediction` / `family_prediction_name`、`results[]` 全候选排名（含 `score`、`probability`）、`ranking_score`、`probability`、`probability_status`、`probability_scope`。

**结果状态机：四态**（设计者判断，回应「误伤好账号」与「沉默失败」两条反目标）

| 状态 | 触发条件 | 计入连续不符计数 | 可触发动账号的动作 | 告警 |
|---|---|---|---|---|
| **相符** | `prediction` == 预期模型 | 否（清零） | 成功动作（见 6.5） | 否 |
| **不符** | `prediction` != 预期模型 | **是** | **是**（满 2 次后） | 是 |
| **无法判定** | `decision` 为 `unscorable` / `not_model` | 否 | **否** | 仅标记，不进 ops 事件 |
| **检测失败** | 请求失败（401/429/5xx/超时/上游拒答）、参考数据加载失败、账号类型不支持 | 否 | **否** | 仅标记，不进 ops 事件 |

两条硬约束：

- **「无法判定」和「检测失败」绝不触发动账号的动作。** 既有 runner 把失败动作绑在 `result.Status == "failed"` 上，而那个 status 包含连通性失败（`backend/internal/service/scheduled_test_runner_service.go:203-250`）。指纹类型**不能**直接复用这条路径——否则一次 429 就会停掉账号，正撞反目标第一条。
- **但它们也不能静默消失。** 界面上必须显示具体原因（哪种失败、什么错误），否则撞反目标第四条。拿不到排名不等于造假，也不等于一切正常。

**预期模型的来源**（设计者判断）：计划表单里做成下拉选择，选项从参考库的 `bundle.json` 动态拉取（含 `id`、`display_name`、`family_name`），新建时按账号的模型名做模糊匹配预选。不要把 66 个模型 ID 硬编码进代码——参考库会随重训更新。

**预期模型只能从参考库里选，不允许自由输入**（设计者判断）。选不到就不让建计划，并说明"参考库暂不支持该模型"。

理由：按 5.4 的封闭集合性质，如果账号跑的模型根本不在参考库里（新模型、微调版），它仍会被硬配一个"最像"的候选，于是**每一轮都判不符**——变成永久误报，正中反目标第一条。从源头禁止建计划，比事后解释误报便宜得多。

**「测试模型」填的是上游实际接收的最终模型名**（设计者判断），即经过本地 `model_mapping` 映射之后的值，而不是用户侧看到的名字。`detect()` 直连上游、绕过本地映射，填错就测不到真实链路。表单上要把这点写清楚。

### 6.4 默认值（需求方认可）

| 项 | 默认值 | 说明 |
|---|---|---|
| cron | 每 6 小时一次 | 不沿用定时测试的 `*/30 * * * *`，成本考量 |
| `count` | 3 | SDK 最高精度；可降到 1 省钱，代价是 `decision` 只能是 `partial`、没有 `probability` |
| 全局并发闸 | 3 | 同一时刻最多并行 3 个指纹检测，避免几百个账号同时开炮 |
| 失败动作 | 全部关闭 | 默认只标记 + 告警，不动账号状态 |
| 接入时校验 | 不勾选 | 批量创建/导入不提供此选项 |
| 邮件告警 | 关闭 | |
| 动账号动作的触发门槛 | 连续 **2** 次不符 | 需求方拍板；告警与标记仍是单次立即出 |
| 每日检测次数上限 | 全局与单账号各一个上限 | 需求方拍板，见 6.9 |

`parallel`（单次检测内部的并发，1–3）、`strict`、`timeout`、`effort`、`service_tier`、`stream` 这些 SDK 参数是否暴露给管理员，由实现者判断；**至少 `count` 要可配**，因为它直接对应成本。

### 6.5 失败动作（需求方拍板）

**门槛：连续 2 次判定不符，才执行动账号的动作**（需求方拍板）。

告警和账号列表标记仍然是**单次不符就立即出**——这一层永远不等。只有会改变账号状态的动作才需要二次确认。SDK 文档建议「重复多轮结合其他证据」，反目标第一条也是误伤好账号，两次判定同样不符能把单次误判挡掉绝大部分。

中间夹一次「相符」则计数清零；「无法判定」和「检测失败」不计数也不清零（见 6.3 状态机）。计划表需要一个连续不符计数字段，runner 维护它。

沿用既有的动作体系，默认全关，管理员可在计划里逐项开启：

- 自动停用账号（`failure_disable`）
- 临时不可调度 N 分钟（`failure_temp_unschedulable_minutes`）
- 移出指定分组（`failure_remove_group_ids`）

**成功动作需要裁剪，不能照搬**（设计者判断）。

既有定时测试承担自动恢复探测的职责，所以 `SuccessEnable` / `SuccessRecover` 会在测试通过时启用账号。但对指纹计划来说，一次"相符"去**启用一个管理员手工停用的账号**是意外副作用——与需求方「停不停我自己决定」的要求直接冲突。

指纹类型默认不提供 `SuccessEnable` / `SuccessRecover`。`SuccessAddGroupIDs` 可以保留。

### 6.6 告警（需求方拍板）

三层，前两层做，第三层做成开关默认关：

1. **账号列表标记** — 账号行上打"指纹存疑"标记，悬停显示最近一次判成了什么模型、置信度多少、什么时候判的
2. **ops 告警事件** — 生成 `OpsAlertEvent`，接入既有的 `backend/internal/service/ops_alerts.go` / `ops_alert_evaluator_service.go` 体系，可在运维告警页查看、静默、标记已处理

   **接法：为指纹不符建一条内置 rule**（设计者判断）。既有的静默是按 `rule_id + platform + group` 维度的，`severity` / `cooldown_minutes` / `notify_email` 也全挂在 rule 上。走无 rule 的裸事件实现最轻，但"能静默"就没了——而静默正是承诺给需求方的能力（避免同一个账号反复刷屏）。建内置 rule 要动 ops 的规则模型和规则页 UI，这个成本认了。

   注意既有事件全部来自指标阈值 evaluator，指纹不符是事件驱动、没有 metric/threshold，内置 rule 的字段要能容纳这种形态。
3. **邮件** — 复用 `notification_email_service.go`，**默认关闭**，避免抽查频繁导致垃圾邮件

**运维监控总开关关闭时的行为**（设计者判断）：`OpsService.CreateAlertEvent` 开头就 `RequireMonitoringEnabled`，而运维监控有全局硬开关（`cfg.Ops.Enabled` + 运行时设置，见 `backend/internal/service/ops_service.go:146-165`）。

- 第 2 层（ops 事件）跟随该开关，关了就不发事件
- **第 1 层（账号列表标记）不依赖该开关，任何情况下都要出**

这样即使运维监控整个关掉，管理员打开账号列表仍能看见"指纹存疑"，不会出现反目标第四条的沉默失败。

### 6.7 系统设置项

新增一组设置（参照 `backend/internal/service/setting_features.go` 等既有模块的写法）：

- 参考数据源 URL 列表（默认官方源 + jsdmirror 镜像）
- 参考数据缓存路径
- 缓存更新检查间隔（`ttl`）
- 全局并发闸上限（**集群级，见下**）
- 全局每日检测次数上限、单账号每日检测次数上限（见 6.9）
- 邮件告警开关

**全局并发闸必须做成集群级**（设计者判断）：项目有 Redis 和 leader lock、支持多副本部署。进程内的闸在 N 个副本下实际是 `3 × N`，等于没限。用 Redis 计数器实现，复用既有 Redis 连接。

### 6.8 非活跃账号不跑定期抽查

**停用 / 已过期 / 临时不可调度状态的账号，定期抽查直接跳过**（设计者判断）。

既有定时测试是故意照跑这些账号的——它要承担自动恢复探测的职责。但指纹检测跑一个已停用的账号纯属烧钱，而且结论对谁都没用。手动触发不受此限制：管理员明确点了按钮就跑，这是他的选择。

### 6.9 成本记账与封顶（需求方拍板）

方案 A 的 `detect()` 直连上游、不经过项目网关，因此**不会产生 `usage_logs` 记录**，这笔花费在账号用量和成本看板里完全隐形，只能去上游账单核对。而 6.4 的全局并发闸只限并行度、不限总量：N 个账号 × 每 6 小时 × 2000–3000 输出 token 是持续开销，并发闸一点都挡不住。

需求方决定**两件都做**：

1. **记录消耗** — 每次检测把估算的输出 token 数（或 SDK 能拿到的实际用量）存进结果记录，并在指纹面板上显示。至少要能回答"这个功能这个月花了多少"
2. **每日次数上限** — 全局一个上限、单账号一个上限，都做成系统设置项。撞上限就跳过本次检测，并**把跳过原因记进结果**（不能静默跳过，撞反目标第四条）

上限的默认值由实现者按合理值定，但必须是一个真能起作用的数——不是设成无穷大走过场。

---

## 7. 留给实现者的判断

以下是设计上没有定死、由实现者按项目惯例决定的部分：

1. **表结构**：扩展 `scheduled_test_plans` 加类型字段，还是另建表共用 runner（见 6.1 的判据）
2. **detector 实例的生命周期管理**：在哪一层持有、何时加载、加载失败如何降级并在界面上暴露原因
3. **账号 → `detect()` 参数的映射**：`base_url` / `api_key` 从 `credentials` 取。注意**没有统一的 `Account.GetAPIKey()`**，实际是按平台分的一组方法：`GetCredential("api_key")`、`GetTypeSafeAPIKey()`（account.go:1144）、`GetCNAPIKey()`（:1674）、`GetOpenAIApiKey()`（:1843）、`GetOpenAIProtocolAPIKey()`（:1855）；base URL 侧有 `GetBaseURL()`、`GetOpenAIBaseURL()`、`GetAnthropicProtocolBaseURL()`、`GetCNProtocolBaseURL()`。`api` 取值如何从账号的 platform / 协议推导（项目已有 `APIProtocolChatCompletions`、`APIProtocolAnthropic` 等常量）
4. **账号类型的可用性判断**（补充：配了 `header_overrides` 或非标准鉴权头的账号，SDK 自建请求大概率直接失败，应一并归入"不支持"类并置灰说明）：哪些 `type` / `platform` 组合算"支持指纹校对"，以及不支持时界面上怎么说明（`AccountTypeUpstream` 和 `AccountTypeAPIKey` 是主要目标；`AccountTypeOAuth`、`AccountTypeSetupToken`、`AccountTypeBedrock`、`AccountTypeServiceAccount` 不支持）
5. **结果存储形态**：完整 `analysis` 存 JSON 列还是拆列；留存条数沿用 `max_results`
6. **前端面板的组织方式**：在 `ScheduledTestsPanel.vue` 里加类型切换，还是新建一个并列的面板组件
7. **i18n**：中英文文案都要补（`frontend/src/i18n/locales/{zh,en}/admin/`）
8. ~~代理~~ **已查证，不再是开放问题**：SDK 的 HTTP 客户端不可注入，按账号配代理无法实现，详见 5.3.1。实现者需要决定的只剩界面上如何呈现这个局限

---

## 8. 参考资料

- SDK 总览：https://lm.ikale.io/docs/sdk/
- API 参数与结果字段：https://lm.ikale.io/docs/sdk/api/
- 参考数据与离线使用：https://lm.ikale.io/docs/sdk/data/
- 参考库清单：https://lm.ikale.io/data/bundle.json
- 仓库：https://github.com/Ikaleio/lm-detector
