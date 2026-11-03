# Model Plaza: localized group labels and hover-only notes

## User request

On the public Model Plaza page, make group names localize with the selected UI language. Add a compact note/help indicator whose explanation is tucked away and appears on hover (and remains accessible to keyboard/touch users). Use Codex to implement this.

## Scope and constraints

- Repository: `/home/debian/ai/sub2api-deploy/src/sub2api`.
- Read `AGENTS.md` first. Follow its commit and no-build/no-compile instructions.
- Limit changes to the public Model Plaza group-name presentation and its user-facing hint, locale strings, and focused tests.
- Do not rename or rewrite group names in the database/API. Preserve the original group name for routing, identity, admin views, and unknown/custom names.
- Do not change prices, rates, multipliers, billing behavior, API contracts, or unrelated group selectors.
- Do not change the pre-existing untracked `NEWAPI_UPSTREAM_RATE_CONTINUATION.md` or any other pre-existing user work. The current task brief is also untracked and must not be included in the application commit.
- Do not add dependencies or touch credentials.
- Do not run build/compile commands. Use allowed non-compiling checks and report omitted tests/builds.

## Implementation guidance

1. Inspect `PlazaGroupSection.vue`, the model-plaza data types and request path, current locale selection, the model-plaza locale keys in Chinese and English, and its existing component tests.
2. Make group-name localization presentation-only and deterministic:
   - Prefer stable group IDs/codes or an existing structured translation field if the public payload provides one.
   - If groups are arbitrary admin-defined names without stable semantic codes, add a small explicit frontend translation map for only verified built-in names; for all unknown/custom names, show the original backend name unchanged.
   - Never translate by guessing fuzzy substring matches, numeric multiplier, array index/order, or provider/model content. Do not fabricate semantic translations for names not confirmed by source.
   - Switching locale should update displayed labels without changing group identity, model membership, links, filters, or pricing.
3. Add a small info affordance beside the group heading. Its note should be hidden in normal layout, appear as a tooltip on hover, and also work on keyboard focus and touch/click with an accessible name. Reuse the existing tooltip/popover primitive if the frontend has one. Put the note copy in both `zh` and `en` locale files. Keep note factual and concise; do not promise provider affiliation, stability, performance, availability, or a price formula that is not verified. Suggested intent: explain that localized group labels are display names only; actual available models and applicable channel/pricing conditions are shown in the catalog/details.
4. Add/update focused tests for: a known localized group name, locale switching, unknown/custom-name fallback, and the hint's accessible content/trigger if the test setup permits.

## Acceptance criteria

- Public Model Plaza group labels use the selected locale where an explicit verified translation exists.
- Unknown/admin-defined group names remain exactly as supplied by the API.
- The tooltip note is hidden until requested by hover/focus/touch and has a keyboard-accessible trigger.
- Changing display language does not alter group IDs, model associations, or price values.
- No other global/admin group-name surface changes.
- Focused non-compiling/static checks allowed by `AGENTS.md` pass; do not build or compile.
- Commit only task-specific application source and test files. Do not add this brief or pre-existing untracked files to the commit. Do not push.
- Final report includes commit hash, changed files, checks, and what was intentionally not changed.
