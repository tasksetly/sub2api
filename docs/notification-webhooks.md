# 充值完成 Webhook

管理员在 **系统设置 → Webhook 通知** 中配置接收地址、开关和可选签名密钥，然后点击本区域的保存。当前使用通用 `POST application/json` 协议，接收端需要按下文处理 JSON；飞书、企微等机器人的专用消息格式暂未接入。

接收地址必须为公网 HTTPS，不支持 URL 用户名密码、片段、私网地址、代理或重定向。地址与签名密钥使用项目现有 AES-GCM 加密器保存，签名密钥不会回显；编辑时留空保持原密钥，勾选清除才会删除。

服务器必须配置固定的 `TOTP_ENCRYPTION_KEY`（64 位十六进制，即 32 字节），所有实例使用同一个值。项目在未配置时会临时生成密钥，每次重启都会改变，因此 Webhook 页面会阻止使用临时密钥保存配置。不要随意轮换这个密钥，它也用于项目的其他加密配置。

## 触发范围

唯一事件为 `payment.recharge.completed`：在线余额充值到账、订单首次进入 `COMPLETED` 时创建通知。支付成功但尚未入账时不触发；订阅购买、兑换码充值、后台手工调整余额和退款不触发。

启用后只处理新完成的订单，不补发历史订单。更改接收地址、签名密钥或开关会生成新配置版本，旧版本尚未发送或等待重试的通知会标记为跳过；已经发出的请求可能仍会完成。保存相同配置不会使队列失效。

“发送测试通知”使用已保存配置，关闭通知时也可以测试。它发送带 `test: true` 的模拟充值事件，订单 ID、用户 ID 均为 `0`，不会创建订单、改变余额或写入正式发送记录。

## 请求示例

```json
{
  "id": "payment.recharge.completed:123",
  "event": "payment.recharge.completed",
  "created_at": "2026-11-15T10:00:00Z",
  "data": {
    "order_id": 123,
    "out_trade_no": "ORDER202611150001",
    "user_id": 10,
    "user_email": "buyer@example.com",
    "user_name": "Buyer",
    "order_type": "balance",
    "status": "COMPLETED",
    "payment_type": "alipay",
    "pay_amount": 68,
    "pay_currency": "CNY",
    "credited_amount": 11,
    "bonus_amount": 1,
    "balance_currency": "USD",
    "paid_at": "2026-11-15T09:59:59Z",
    "completed_at": "2026-11-15T10:00:00Z"
  }
}
```

`pay_amount` 是用户实际支付的金额，`pay_currency` 来自订单保存的支付渠道币种，缺失或无效时为 `CNY`。`credited_amount` 是到账美元额度，**已包含** `bonus_amount`；余额币种固定为 `USD`。用户信息取订单快照。接收端应使用十进制定点数处理金额，不要将不同币种相加。

所有请求均含这些请求头：

| 请求头 | 内容 |
| --- | --- |
| `X-Webhook-Id` | 与 JSON 的 `id` 相同，重试保持不变 |
| `X-Webhook-Event` | `payment.recharge.completed` |
| `X-Webhook-Timestamp` | 本次发送的 Unix 秒数，重试时重新生成 |
| `X-Webhook-Signature` | 配置密钥后才发送：`sha256=` 加 HMAC-SHA256 十六进制摘要 |

签名原文是时间戳、英文句点和**原始请求体字节**拼接，不能把 JSON 解析后重新序列化再验签。以下 Node.js 示例只使用标准库：

```js
import { createHmac, timingSafeEqual } from 'node:crypto'

// rawBody 为接收框架保留的原始 Buffer，headers 为收到的请求头。
function verifyWebhook(rawBody, headers, secret) {
  const timestamp = headers['x-webhook-timestamp']
  const signature = headers['x-webhook-signature']
  if (typeof timestamp !== 'string' || !/^\d+$/.test(timestamp)) return false
  if (typeof signature !== 'string' || !/^sha256=[0-9a-f]{64}$/.test(signature)) return false
  if (Math.abs(Date.now() / 1000 - Number(timestamp)) > 300) return false
  const expected = createHmac('sha256', secret)
    .update(timestamp + '.').update(rawBody).digest()
  const actual = Buffer.from(signature.slice(7), 'hex')
  return actual.length === expected.length && timingSafeEqual(actual, expected)
}
```

验签通过后，接收端应校验事件类型和 `test` 标记，再按 `id` 幂等处理。事件 ID 只在当前站点内唯一；多个站点共用接收器时，需要分别标识站点。各端保持时钟同步，以便校验时间戳。

## 发送、重试与记录

订单完成状态和事件在同一数据库事务内落库，后台工作协程再发送 HTTP 请求。接收端故障不阻塞充值请求等待网络响应；服务重启后可继续处理持久化队列。此设计保证成功提交的完成状态对应已入队事件，因此数据库入队失败时，完成状态更新也会回滚，由支付流程后续重试。

接收端返回任意 `2xx` 表示成功；其他状态（包括 `3xx`）或网络失败均视为失败。每次 HTTP 请求超时为 10 秒，失败后依次等待约 1 分钟、5 分钟、15 分钟、1 小时，最多尝试 5 次，随后标记失败。正常情况下后台每 5 秒检查待发送任务；积压或重启恢复可能增加等待时间。

每条任务通过数据库租约领取，避免多个实例正常运行时重复发送；租约在 60 秒后可回收。若接收端已处理但响应丢失，或者发送后进程退出而未保存结果，仍可能重复发送。**请按固定事件 ID 去重**，并让重复请求也返回 `2xx`。

设置页显示最近 20 条正式通知的状态、尝试次数和最近 HTTP 结果；已结束记录保留约 30 天。失败后不提供手动重放；修改设置也不会补发旧事件。日志与发送记录不保存接收 URL、签名密钥或接收端响应正文。

## 部署与开发验证

本功能包含迁移 `266_notification_webhooks.sql`，部署时由项目现有迁移流程创建发送队列和订单完成触发器。滚动更新时，旧服务产生的完成订单也能由新服务处理通知。不要只更新前端而遗漏后端和迁移。

已增加服务单元用例、数据库集成用例和审计脱敏用例，覆盖签名、密钥保留/清除、URL 防护、重试、配置版本、触发范围、订单事务回滚和租约恢复。遵循仓库约定，本次仅做静态检查，未执行构建、测试、数据库迁移或真实 Webhook 请求。
