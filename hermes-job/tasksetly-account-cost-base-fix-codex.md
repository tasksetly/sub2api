# Fix account-cost base selection for owner-confirmed PoJia bug

## Scope and authorization
Owner explicitly asks Codex to fix the local cost algorithm. Implement source changes and a local Git commit only; do not push, deploy, restart containers, access production credentials, mutate live configuration, run migrations, or rewrite historical ledgers. Read AGENTS.md and git status first. Preserve unrelated/untracked files; do not commit this brief. NO builds, go test, go generate, frontend build, or compiling tests. Write focused regression tests before implementation, but report Go tests NOT RUN under the no-compilation rule. Use actual non-compiling checks where applicable, not a duplicated Python formula as a fake test of Go code.

## Exact reproduced bug (verified DB read)
request_id: client:ec5e7c1e-1432-4616-bc62-52c8fa1e6c33
usage id 600325; account 278 PoJia/破甲 DS 无甲 已更新1017; provider 24; group 44; channel 8 ds.
model/upstream_model/requested_model all deepseek-v4.1-flash.
Input=32, output=14, cache=0; timestamp 2026-10-31T15:27:19.529445Z.
Channel configured USD/token input .000002, output .000008, cache .00000004, matching upstream bound group's price ($2/$8/$.04 per million).
Local fields:
- input_cost .000064; output_cost .000112
- total_cost .000176
- account_stats_cost .0000132
- account_rate_multiplier .25
- rate_multiplier .45 (customer billing rate, DO NOT change or confuse with cost rate)
- actual_cost .0000792 (customer debit, DO NOT change)
- ledger account_cost .0000033
Correct account cost for this scenario: .000176 * .25 = .000044.
Owner said '按0.44的算法' in context of the preceding .000044 explanation: do NOT interpret this as a literal $0.44 price or change .25 to .44, do not hardcode this request/model/group/rate or rescale currency.

## Root cause verified
backend/internal/service/account_stats_pricing.go resolveAccountStatsCost priority:
1 explicit AccountStatsPricingRules
2 ApplyPricingToAccountStats=true uses pre-multiplier totalCost
3 local model file/default official pricing
4 nil fallback to total_cost
Channel 8 apply_pricing_to_account_stats=false and there are no explicit account-stat custom rules in live DB. Account stats silently uses official DeepSeek default prices ($.15/$.6/$.003 per million) instead of configured channel prices. That produces .0000132 rather than .000176, then multiplies by .25. This is not floating-point rounding or wrong multiplication. Relevant constants are in billing_service.go.
Frontend components/admin/usage/UsageTable.vue shows total_cost and account rate but accountBilled computes (account_stats_cost ?? total_cost)*account_rate_multiplier, hiding the substituted cost base.
DB trigger in migrations/258_upstream_cost_ledger.sql uses same formula; repo dashboard stats use same formula.

## Desired behavior
For normal locally configured token pricing without an explicit account-cost override, account-cost base should follow the actual pre-multiplier billed base total_cost (matching owner-aligned upstream configured unit prices), then multiply exactly once by request account_rate_multiplier. Do not silently substitute official/default prices merely because a group has a channel whose legacy ApplyPricingToAccountStats flag is false. This fix must work without manually changing production channel 8 settings.

Preserve deliberate explicit account-stat custom pricing rules as overrides. Inspect all call paths, tests, historical flag semantics, model mapping and long-context behavior before selecting the smallest safe policy change. Avoid removing intentional separate upstream cost calculation for genuinely different mapped models or explicit account-specific pricing controls without justification; if necessary scope the new default to same billed/upstream model + configured pricing and make the decision explicit. Ensure the exact same-model PoJia bug is fixed when ApplyPricingToAccountStats is false. Do not solve this by disabling all cost overrides in aggregation or by writing a migration that updates live settings.

Do not change customer total_cost, customer actual_cost, group customer multiplier, cache parsing, provider rates, payment behavior, or currencies. Do not add hardcoded provider/account IDs.

Add regression coverage for input32/output14/channel 2e-6+8e-6 with account rate .25 resulting .000044 rather than .0000033; disabled legacy toggle; explicit account cost override preserved; normal default path; distinct mapped-model policy if affected. Inspect relevant existing account_stats_pricing tests and add tests FIRST. Add frontend cost-base display and localized explanation if needed so explicit overrides no longer look like bad multiplication; keep minimal and use existing format/currency conventions.

Historical logs and immutable cost ledger entries remain unchanged. Identify the repair boundary clearly in the final report; do not claim source changes repair record600325 before deployment/backfill. Do not create/apply a historical repair script unless reporting-only and explicitly necessary.

## Checks and deliverable
Run git diff --check, gofmt on edited Go files, and targeted existing non-compiling lint if available. No compiling test/build commands. Inspect changed callsites and snapshots statically. Create a local commit for only task files. Report exact chosen cost-base semantics, changed paths, regression cases added, executed checks with real output, skipped checks, commit hash, and any compatibility concerns. No fabricated RED/GREEN claim if regression Go tests cannot be executed.
