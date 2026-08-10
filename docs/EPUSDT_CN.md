# Epusdt EPay 接入

Sub2API 可通过现有的 EasyPay provider 对接 Epusdt 的 EPay 兼容收银台。Epusdt 的该接口是跳转收银台，不提供传统 EasyPay 的 `mapi.php`、`api.php` 查单或退款接口。

## Epusdt 配置

1. 在 Epusdt 管理端创建或确认一个启用的 API Key。
2. 使用数字 PID，例如 `1000`。Epusdt 的 EPay 回调会将 PID 转成数字，非数字 PID 无法生成回调。
3. 记录该 API Key 的 `secret_key`。它是 Sub2API EasyPay 配置中的 `PKey`，不是旧版环境变量 `epay_key`。
4. 为 EPay 设置默认资产，例如 `epay.default_token=usdt`、`epay.default_network=tron`、`epay.default_currency=cny`。未设置 token/network 时，Epusdt 会创建由用户在收银台选择网络的占位订单。
5. EPay 下单由最终用户的浏览器发起。若 API Key 设置了 IP 白名单，白名单会校验用户浏览器的公网 IP，而不是 Sub2API 服务器 IP。通常应留空该白名单。

## Sub2API Provider 配置

在“管理后台 -> 支付设置 -> 服务商管理”新建 EasyPay provider：

| 字段 | 值 |
| --- | --- |
| 服务商名称 | `Epusdt EPay` |
| 支付模式 | 任意（Epusdt 路径会自动使用当前页面跳转收银台） |
| 支持方式 | 仅启用 `alipay` |
| PID | Epusdt API Key 的数字 PID，例如 `1000` |
| PKey | 同一 API Key 的 `secret_key` |
| API 基础地址 | `https://<epusdt-domain>/payments/epay/v1/order/create-transaction` |
| 异步通知地址 | `https://<sub2api-domain>/api/v1/payment/webhook/easypay` |
| 同步跳转地址 | `https://<sub2api-domain>/payment/result` |

`API 基础地址` 也可以填写完整的 `.../submit.php` 地址。不要只填写 Epusdt 域名，否则 Sub2API 会拼出普通 EasyPay 的 `/submit.php` 路由，无法命中 Epusdt 的兼容接口。

Epusdt 的 `type=alipay` 是 EPay 兼容值，不表示必须使用支付宝。它会按 Epusdt 的默认 token/network 创建链上支付订单。

## 可选网络选择器

Epusdt 也接受 `type=usdt.tron` 这样的 `token.network` selector。若要在 Sub2API 里展示独立方式，添加 EasyPay 自定义方式：

| 字段 | 示例 |
| --- | --- |
| 支付方式 | `usdt_tron` |
| 上游 type | `usdt.tron` |
| 显示名称 | `USDT-TRON` |

本地支付方式使用下划线，上游 type 使用 Epusdt 要求的点号格式。不要启用内置 `wxpay`，因为 Epusdt EPay 会拒绝 `type=wxpay`。

## 回调与验证

Epusdt 支付成功后会向上述异步通知地址发起 GET 请求，携带 `out_trade_no`、`trade_no`、`money`、`trade_status=TRADE_SUCCESS`、`sign` 和 `sign_type`。Sub2API 已提供该路由，并使用同一个 `secret_key` 按 EPay MD5 规则验签，成功后返回纯文本 `success`。

确认两端均可从公网访问：Epusdt 必须能访问 Sub2API 的 HTTPS 回调地址，且反向代理不能拦截 `/api/v1/payment/webhook/easypay` 的 GET 请求。Sub2API 会在当前页面跳转到 Epusdt 收银台，不会额外打开支付窗口。不要依赖 Epusdt EPay 的主动查单或退款；保持该 provider 的退款开关关闭。

## 排查

| 现象 | 原因与处理 |
| --- | --- |
| 请求落到 `/mapi.php` 或 `/api.php` | 使用 Epusdt 专用 API 基础地址并升级到包含本兼容修复的版本。 |
| Epusdt 返回 `10009 invalid params` | 仅使用 `alipay` 或已启用的 `token.network`，例如 `usdt.tron`；不要发送 `wxpay`。 |
| 签名错误 | PID 和 `secret_key` 必须来自同一个已启用 API Key；不要使用旧 `epay_key`。 |
| 用户无法打开收银台 | 检查 API Key 的 IP 白名单。redirect 下单会使用用户浏览器 IP。 |
| 支付完成但余额未到账 | 检查 Epusdt 回调日志、Sub2API webhook 日志、HTTPS 可达性，以及回调响应是否为 HTTP 200 和 `success`。 |
