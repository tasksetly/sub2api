# Codex CLI 怎么接入自定义 API：从注册到第一次调用

这篇只解决一个问题：把 Codex CLI 接到一个 OpenAI 兼容的 API 地址，并完成一次真实请求。

示例使用 Tasksetly，地址是 <https://ai.tasksetly.com>。换成其他兼容服务时，步骤也基本一样。

## 先准备三样东西

1. 已安装 Codex CLI，并且在终端执行 `codex --help` 能看到帮助信息；
2. 一个已经验证邮箱的 Tasksetly 账号；
3. 在控制台创建的 API Key。

如果想先试用，注册并验证邮箱后，在站内工单申请 3 USD 兑换码。工单由人工处理，规则是 24 小时内处理。

## 配置 API 地址和 Key

先在当前终端设置环境变量：

```bash
export OPENAI_BASE_URL="https://ai.tasksetly.com/v1"
export OPENAI_API_KEY="你的 Tasksetly API Key"
```

不要把真实 Key 直接写进 Git 仓库，也不要把它贴到工单、截图或公开文章里。

如果你使用的是 Windows PowerShell，对应写法是：

```powershell
$env:OPENAI_BASE_URL = "https://ai.tasksetly.com/v1"
$env:OPENAI_API_KEY = "你的 Tasksetly API Key"
```

## Pi Agent 配置

Pi Agent 使用 `~/.pi/agent/models.json` 管理自定义模型。先安装 Pi：

```bash
npm install -g --ignore-scripts @earendil-works/pi-coding-agent
```

然后设置 Tasksetly API Key，并创建 Pi 的模型配置目录：

```bash
export TASKSETLY_API_KEY="你的 Tasksetly API Key"
mkdir -p ~/.pi/agent
```

在 `~/.pi/agent/models.json` 写入以下内容。将 `MODEL_ID_FROM_DASHBOARD` 替换成控制台中当前可用的精确模型 ID，例如 GPT 5.4 或 GPT 5.6 对应的实际 ID：

```json
{
  "providers": {
    "tasksetly": {
      "baseUrl": "https://ai.tasksetly.com/v1",
      "api": "openai-completions",
      "apiKey": "$TASKSETLY_API_KEY",
      "authHeader": true,
      "models": [
        {
          "id": "MODEL_ID_FROM_DASHBOARD",
          "name": "Tasksetly GPT",
          "reasoning": true,
          "input": ["text"]
        }
      ]
    }
  }
}
```

配置完成后进入你的项目目录并启动 Pi：

```bash
cd /path/to/project
pi
```

在 Pi 内输入 `/model`，选择 `tasksetly` 下刚配置的模型。`models.json` 在每次打开 `/model` 时会重新读取，因此修改模型 ID 后不需要重装 Pi。

如果你的服务端不兼容 OpenAI Chat Completions 的 `developer` 角色或 `reasoning_effort` 参数，需要在 `tasksetly` Provider 下补充相应的 `compat` 配置；先以默认配置完成一次真实调用，再根据实际报错调整。

不同版本 Codex CLI 读取的配置方式可能不同。环境变量是最容易先验证的一种方式；如果你的版本没有读取 `OPENAI_BASE_URL`，优先查看 `codex --help` 和控制台提供的配置片段，不要照抄别人的配置文件。

## 先做一个小测试

```bash
codex --help
codex
```

在 Codex 里输入一个不涉及敏感代码的小任务，例如：

```text
请解释当前目录下 README.md 的目录结构，不要修改文件。
```

第一次测试的目的不是让它完成复杂开发，而是确认四件事：

- CLI 能读到 API Key；
- Base URL 指向了预期地址；
- 选择的模型在控制台确实可用；
- 控制台出现了对应的调用和费用记录。

## 常见问题

### 返回 401

通常先检查 Key 是否复制完整、环境变量是否在当前终端生效：

```bash
printf '%s' "$OPENAI_BASE_URL"
test -n "$OPENAI_API_KEY" && echo "API key is set"
```

不要用 `echo` 把完整 Key 打到共享终端或日志里。

### 返回 404 或模型不存在

先到控制台查看精确模型 ID。文章、截图和旧配置里的模型名可能已经过时，模型名不要凭记忆填写。

### 请求成功但余额没有变化

先等几秒刷新用量页面，再核对是否真的是 Tasksetly 的 Key 和 Base URL。后台测试请求和真实客户端请求也可能显示在不同的明细里。

## 计费怎么理解

当前按余额消费：充值 1 CNY 折算 1 USD 余额，Token 消耗从模型基准价格的 0.1 倍起。当前支持 GPT 5.4、GPT 5.6 及以上版本。具体输入、输出、缓存和模型价格，以控制台当前显示为准，页面的价格更新时间也会跟着调整。

更新时间：2026-08-04
