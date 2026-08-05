# Tasksetly 推广渠道与发帖教程

> 更新时间：2026-08-05
>
> 目标：用真实接入教程和用户反馈验证 Tasksetly 的首条增长链路，而不是先购买泛流量。
>
> 相关内容：[项目故事](01_v2ex_project_story_cn.md)｜[Codex CLI 接入](02_codex_cli_config_cn.md)｜[价格与体验码](03_pricing_and_trial_cn.md)

## 1. 首期定位

首期只推广一个场景：**中国开发者把 Codex CLI 接入到一个按量计费的 OpenAI 兼容 API 地址，并完成第一次真实调用。**

主要面向：

- 已经在使用 Codex CLI 的个人开发者；
- 偶尔使用 AI 编程工具、不想长期购买固定订阅的人；
- 需要查看余额、调用次数和 Token 消耗的人；
- 需要统一管理 API Key 的小型研发团队。

当前公开口径：

- 余额按量付费；
- 充值 1 CNY，折算 1 USD 余额；
- Token 消耗按模型基准价格 0.1 倍起；
- 具体可用模型、输入价格、输出价格和缓存价格以控制台当前显示为准；
- 注册并完成邮箱验证后，可以在站内工单申请 3 USD 体验兑换码；
- 体验码由人工处理，预计 24 小时内处理。

发布前重新核对价格、模型 ID、服务地区、退款规则和客服承诺。文案中不使用无法由当前配置证明的“官方”“全网最低价”“永不宕机”等表述。

## 2. 渠道优先级

### P0：Linux.do

适合发布技术型的真实体验帖，主题围绕：

- Codex CLI 配置和首次调用；
- 自定义 Base URL；
- 401、404、模型不存在等排错；
- 按量计费和调用明细是否容易理解。

首帖使用“找人帮忙跑一遍”的项目故事，不要直接使用价格广告标题。评论区及时回答配置问题，并公开说明自己是服务部署者。

### P0：V2EX

首发位置优先选择“分享创造”类主题。内容应包含项目背景、技术路径、当前限制和希望收集的反馈。

在程序员、问与答等主题中，只回复确实在询问 Codex CLI API、OpenAI 兼容接口或按量计费的问题。先给出解决步骤，再附一条教程链接。

### P1：掘金

发布完整接入教程，不要把文章写成服务介绍。推荐标题：

```text
Codex CLI 怎么接入自定义 API：从环境变量到第一次调用
```

文章主体使用 [02_codex_cli_config_cn.md](02_codex_cli_config_cn.md)，先讲通用配置，再在结尾说明 Tasksetly 的示例地址和体验流程。

### P1：B 站

录制 2-3 分钟的真实操作：注册、验证邮箱、创建 API Key、复制配置、完成一次简单调用、查看用量记录。视频中不要展示完整 API Key、邮箱或用户数据。

推荐标题：

```text
Codex CLI 接入自定义 API，3 分钟完成第一次调用
```

### P2：GitHub

只在自己的仓库、Fork、Release 或 Discussion 中发布部署说明、配置模板和版本更新。不要在无关项目的 Issue 中投放服务链接。

## 3. 应该寻找什么帖子

优先找“用户正在解决问题”的帖子，而不是泛泛的广告汇总帖。站内搜索或搜索引擎使用以下关键词：

```text
Codex CLI 怎么配置 API
Codex CLI 自定义 Base URL
Codex API 中转
OpenAI 兼容接口
AI 编程工具 按量付费
Codex 401
Codex 404 模型不存在
国内 Codex API 接入
```

适合回复的帖子通常有以下特征：

1. 发帖人明确说自己正在使用 Codex CLI；
2. 发帖人正在询问自定义 API、模型 ID 或 Base URL；
3. 发帖人正在比较订阅制和按量付费；
4. 发帖人遇到了 401、404、余额或调用记录问题。

回复结构固定为：**先给可执行步骤 → 说明一个常见坑 → 最后附教程链接。** 不要只回复“注册送额度”或“私聊”。

## 4. 72 小时发布顺序

### 第 1 天：一个社区首帖

在 Linux.do 或 V2EX 选择一个平台首发，不要同时复制两篇完全相同的内容。

目标是收集 5-10 个真实测试反馈，重点记录：

- 从注册到首次调用的耗时；
- 首次调用失败的原因；
- 用户是否找到准确模型 ID；
- 用户是否看懂余额和 Token 扣费；
- 是否需要人工客服介入。

### 第 2 天：回复真实问题

搜索上述关键词，回复 3-5 个相关问题。每条回复使用不同的 `utm_content`，并在评论中回答后续追问。

### 第 3 天：发布接入教程

将教程发布到掘金，或把录屏发布到 B 站。教程只解决“如何完成第一次调用”，不要同时讲所有模型、所有工具和所有优惠。

## 5. UTM 链接

每个平台和每篇内容使用单独链接，便于回查注册、首次成功调用和首付费：

```text
https://ai.tasksetly.com/go/codex?utm_source=v2ex&utm_medium=community&utm_campaign=codex_launch_202608&utm_content=project_story

https://ai.tasksetly.com/go/codex?utm_source=linuxdo&utm_medium=community&utm_campaign=codex_launch_202608&utm_content=project_story

https://ai.tasksetly.com/go/codex?utm_source=juejin&utm_medium=article&utm_campaign=codex_launch_202608&utm_content=setup_guide

https://ai.tasksetly.com/go/codex?utm_source=bilibili&utm_medium=video&utm_campaign=codex_launch_202608&utm_content=setup_video
```

发布前把链接登记到 `growth_daily`，至少记录来源、发布时间、内容名称和负责人。首期重点观察：

```text
落地页访问 -> 注册 -> 邮箱验证 -> 创建 Key -> 首次成功调用 -> 第三次调用 -> 首付费
```

## 6. V2EX / Linux.do 首发帖

### 标题

```text
基于开源项目部署了一个 Codex CLI API 服务，想找人帮我跑一遍
```

### 正文

````markdown
大家好，我是 Tasksetly 这个线上服务的部署者。

最近基于开源项目部署了一套 Codex CLI API 服务，首期只围绕 Codex CLI 做接入，
现在想找一些真实用户帮忙跑一遍完整流程。

官网：
https://ai.tasksetly.com/go/codex

目前的规则：

- 余额按量付费；
- 充值 1 CNY，折算 1 USD 余额；
- Token 消耗按模型基准价格 0.1 倍起；
- 具体可用模型以控制台当前显示为准；
- 注册并完成邮箱验证后，可以提交站内工单申请 3 USD 体验兑换码；
- 体验码由人工处理，预计 24 小时内处理。

如果你正在使用 Codex CLI，欢迎帮忙验证：

1. 注册、验证邮箱和创建 API Key 是否容易理解；
2. 按页面配置后能否完成第一次真实调用；
3. 余额和 Token 消耗是否看得懂；
4. 401、404 或模型错误时，页面提示是否足够；
5. 从注册到首次调用大概花了多长时间。

比起“挺好用”，更希望收到具体反馈，例如哪一步找不到、模型 ID 是否容易填错、
扣费明细是否看得懂。这些反馈会直接用于修改页面和接入文档。

欢迎直接评论，也可以注册后在站内提交工单申请体验码。
````

发布时只保留一个主链接。价格、模型和体验码规则发生变化时，先修改正文再继续投放。

## 7. Codex CLI 接入教程帖

### 标题

```text
Codex CLI 怎么接入自定义 API：从环境变量到第一次调用
```

### 正文结构

````markdown
这篇只解决一个问题：把 Codex CLI 接到一个 OpenAI 兼容的 API 地址，并完成一次真实请求。

## 准备

1. 已安装 Codex CLI，并确认 `codex --help` 可以正常运行；
2. 一个已经验证邮箱的账号；
3. 控制台创建的 API Key。

## 配置

Linux/macOS：

```bash
export OPENAI_BASE_URL="https://ai.tasksetly.com/v1"
export OPENAI_API_KEY="你的 API Key"
```

Windows PowerShell：

```powershell
$env:OPENAI_BASE_URL = "https://ai.tasksetly.com/v1"
$env:OPENAI_API_KEY = "你的 API Key"
```

不要把真实 API Key 写入 Git 仓库、截图、工单或公开评论。

## 首次验证

```bash
codex --help
codex
```

进入 Codex 后，先输入一个不涉及敏感代码的小任务：

```text
请解释当前目录下 README.md 的目录结构，不要修改文件。
```

首次测试主要确认：

- CLI 读取到了 API Key；
- Base URL 指向了预期地址；
- 使用的模型 ID 在控制台可用；
- 控制台出现了调用和费用记录。

## 常见错误

### 401

检查 API Key 是否复制完整，以及环境变量是否在当前终端生效：

```bash
printf '%s' "$OPENAI_BASE_URL"
test -n "$OPENAI_API_KEY" && echo "API key is set"
```

### 404 或模型不存在

到控制台复制精确模型 ID。不要直接使用旧文章、旧截图或凭记忆填写模型名。

### 调用成功但余额未更新

等待几秒后刷新用量页面，再确认使用的是对应的 API Key 和 Base URL。

## 示例服务

本文使用 Tasksetly 作为示例，官网：
https://ai.tasksetly.com/go/codex

注册并验证邮箱后，可以在站内工单申请 3 USD 体验兑换码。
````

教程正文先解决通用技术问题，服务介绍放在结尾。Pi Agent 配置单独拆成第二篇，避免首篇过长。

## 8. 评论回复模板

````markdown
如果你只是要验证 Codex CLI 是否支持自定义 API，可以先设置：

```bash
export OPENAI_BASE_URL="https://ai.tasksetly.com/v1"
export OPENAI_API_KEY="你的 API Key"
codex --help
```

然后启动 Codex 做一次简单调用。常见问题是模型 ID 不准确，先复制控制台当前显示的精确名称。

完整的配置和 401/404 排查步骤：
https://ai.tasksetly.com/go/codex?utm_source=COMMENT_SOURCE&utm_medium=community&utm_campaign=codex_launch_202608&utm_content=setup_reply
````

## 9. 发布前检查

- 首页第一屏只突出 Codex CLI，不要同时把 Claude、GPT、Gemini 都当成首期主推产品；
- “注册即可获得免费试用”改成“验证邮箱后提交工单申请体验码”；
- 页面展示精确模型价格、计费单位和最后核验时间；
- 注册、邮箱验证、创建 Key、首次调用和用量记录完整跑通；
- 不在截图、视频、日志和文章中暴露真实 API Key；
- 每个平台使用唯一 UTM；
- 发布后 2 小时内安排人员回答配置问题；
- 记录首次调用、第三次调用、首付费和退款，不用阅读量替代激活指标。

## 10. 停止扩散条件

出现以下情况时，先修产品和客服流程，再继续发布：

- 注册量增加但没有首次成功调用；
- 首次调用大量返回相同的 401、404 或模型错误；
- 用户无法理解实际扣费；
- 出现重复扣费、退款或体验码滥用问题；
- 页面价格、模型或体验规则与后台实际配置不一致。

## 11. 本次执行记录

- 已用 Windows OpenCLI 登录态在 V2EX「分享创造」发布 1 篇首帖：<https://www.v2ex.com/t/1232182>；
- 发布前已完成 UTF-8 正文复核，发布后确认标题、正文、链接和标签显示正常；
- 本轮不再追加其他社区发帖、无关回复、点赞、关注或私信，先观察真实反馈。
