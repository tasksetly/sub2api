# Tasksetly pricing and positioning cleanup

## Goal

Update the public landing/auth/model-pricing presentation so a first-time user can tell:

- who Tasksetly is for;
- what the first action is;
- that the displayed model price has an upstream reference price, a platform display currency, a channel multiplier, and model-specific conditions;
- that the currently published pricing data must not be silently reinterpreted as a universal formula.

This task does **not** include privacy, terms, refund, or status-page work. The owner will provide that information separately.

## Repository rules

- Work only in this repository: `/home/debian/ai/sub2api-deploy/src/sub2api`.
- Keep the pre-existing untracked file `NEWAPI_UPSTREAM_RATE_CONTINUATION.md` untouched and out of the commit.
- Read `AGENTS.md` before editing.
- Do not change actual model rates, channel multipliers, balance conversion, or billing behavior in this task. First make the UI labels and explanatory copy honest and explicit. Any numerical pricing correction requires a separate, source-backed task after the owner confirms the backend formula.
- Do not add credentials, API keys, or secrets.
- Run non-compiling checks such as `git diff --check` and relevant static/unit checks that do not compile the project. Do not run a build or compilation command unless explicitly requested.
- Create a local Git commit containing only this task's changes. Do not push.

## Verified source facts to preserve

The official price table is not one universal currency/formula:

- Kimi K3: official USD input $3 and output $15 per million tokens.
- Kimi K3.1 Pro: official USD input $2 and output $10 per million tokens.
- GLM-5.3: official CNY input 8 and output 28 per million tokens.
- DeepSeek V4.1 Flash 1017: off-peak input $0.15/output $0.60; peak input $0.30/output $1.20. Official API model name is `deepseek-flash`; the version is `DeepSeek-V4.1-Flash-1017`.
- Grok 4.5/4.6: below 200K prompt tokens input $2/output $6; at or above 200K input $4/output $12. Cached-input rates differ by model.
- Claude Haiku 5.5: up to 100K input tokens $0.10/$0.50 input/output; over 100K $0.50/$2.50.
- OpenAI GPT-5.6 Sol: short-context input $4/output $20; long-context input $8/output $30; cached-input and cache-write prices are separate.

Primary sources:

- https://platform.kimi.ai/docs/pricing/chat
- https://docs.bigmodel.cn/cn/guide/start/pricing
- https://api-docs.deepseek.com/quick_start/pricing
- https://docs.x.ai/developers/models/grok-4.6
- https://docs.x.ai/developers/models/grok-4.5
- https://platform.claude.com/docs/en/about-claude/pricing
- https://developers.openai.com/api/docs/pricing
- https://developers.openai.com/api/docs/models/gpt-5.6-sol.md

## Required changes

### 1. Landing page positioning

Update Chinese and English landing copy in the locale source so that the hero says what the product does without unsupported superlatives:

Chinese direction:

- Main message: API access for AI coding-tool users with usage-based billing.
- First action: sign up, create an API key, read the quickstart, and make a first call.
- Describe the product as one API key/dashboard for the available models; do not imply every model, client, or feature is universally compatible.
- Replace the vague `5.6+` tag with a neutral link/label that points users toward the available-model catalog or documentation.
- Replace `Token 0.1 倍起` in the hero tag with a neutral pricing-directory label such as `查看模型与通道价格`; do not advertise a lowest multiplier until its scope is explicit.
- Keep the existing confirmed balance statement only if it is still true, but clarify that platform balance units are not foreign-exchange rates.
- Replace the pricing feature description with wording that distinguishes upstream reference price, display currency, channel multiplier, input/output/cache pricing, and long-context/peak-time conditions.
- Remove or soften unverified claims such as automatic failover, multi-account pools, universal free model switching, or guaranteed stability. Prefer `查看当前可用模型`, `按需选择可用通道`, and similar bounded wording.

English direction:

- Use `API access for AI coding tools` and usage-based billing.
- Say `Create an API key, follow the quickstart, and make a first request.`
- Use `View model and channel pricing` instead of `Token Rates from 0.1x`.
- Explain that the model catalog contains the current availability and pricing conditions.
- Do not claim universal compatibility, guaranteed stability, automatic failover, or savings that have not been measured.

Keep the existing i18n key structure where practical. Update associated landing tests if they assert the old strings.

### 2. Registration/auth subtitle

Replace the fallback `Subscription to API Conversion Platform` with a clear, consistent subtitle such as:

- Chinese: `面向 AI 编程工具用户的 API 接入服务`
- English: `API access for AI coding tools`

Trace all source-level fallbacks/defaults used by the public registration/auth page. Update the frontend fallback, backend public-settings fallback, admin placeholder/default if appropriate, and tests/docs that are explicitly describing the same default. Do not write a database migration or overwrite a deployed admin setting unless the codebase clearly requires it for the fallback to take effect.

### 3. Pricing table labels and explanation

Locate the model-plaza table and add a concise explanatory note near the table. The wording must make these columns distinct:

- upstream official reference price and its original currency;
- Tasksetly displayed/paid price and its currency/unit;
- channel multiplier;
- input, output, cache read/write, context-length, peak/off-peak, or other applicable conditions.

Use wording equivalent to:

> 模型价格以对应上游 API 价格为参考。页面分别显示上游原币种基准价和人民币实付价；美元与人民币基准不能直接混作同一列。输入、输出、缓存及长上下文/高峰时段可能分别计价，具体条件见模型详情和价格来源。

English equivalent:

> Model pricing references the corresponding upstream API price. The catalog separates the upstream reference currency from the Tasksetly displayed price. Input, output, cache, context-length, and peak/off-peak conditions may be priced separately; see the model details and source link.

Do not hard-code a fixed `6.8` exchange rate or invent a universal formula in this task. If the UI currently calls a converted value `官方价格`, rename the label to something accurate such as `上游基准价` or `参考价（原币种）`, while retaining the actual source currency.

If adding source links or update timestamps is already supported by the model-plaza data shape, expose them. Otherwise leave a narrowly scoped TODO in this task file rather than inventing data fields.

### 4. Documentation/SEO consistency

Search the public landing, registration, model-plaza, Codex landing, and marketing documentation for the exact old claims below:

- `Token 0.1 倍起`
- `Token Rates from 0.1x`
- `Subscription to API Conversion Platform`
- `多模型随意切换`
- `Multi-account pool, auto-failover`

Update only copies that are clearly public positioning for the same product. Preserve historical research notes and dated campaign records; do not rewrite evidence logs as if they were current UI.

## Acceptance criteria

- Chinese and English landing copy no longer presents `0.1x` as a universal headline claim.
- Registration/auth fallback wording is consistent with the landing page.
- Model-plaza users can distinguish upstream reference price/currency from Tasksetly displayed price/currency and see that cache/context conditions may differ.
- No billing calculation, model rate, database setting, or credential is changed.
- Existing i18n/static tests are updated for intentional copy changes.
- `git diff --check` passes.
- `git status` shows the pre-existing `NEWAPI_UPSTREAM_RATE_CONTINUATION.md` remains uncommitted and untouched.
- Commit only the files changed for this task; report the commit hash and skipped build/compile checks.
