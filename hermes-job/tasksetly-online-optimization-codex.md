# Tasksetly online product optimization

## Context

Repository: `/home/debian/ai/sub2api-deploy/src/sub2api`
Live public URLs:
- `https://tasksetly.com`
- `https://tasksetly.com/model-plaza`
- `https://tasksetly.com/docs/quickstart.html`
- API gateway host: `https://ai.tasksetly.com`

A read-only live audit found:
- The public site currently exposes the generic subtitle `Subscription to API Conversion Platform`; the public settings also have empty `home_content` and empty `doc_url`.
- The public model plaza is available without authentication and currently exposes many internal-sounding group labels such as `福利`, `满血版`, `不降智`, and `企业级`.
- `docs/quickstart.html` has a malformed Anthropic curl header: `-H "x-api-key: ***` is missing the closing quote. It also uses placeholder model IDs, which is appropriate because models change, but the copy should clearly explain where to copy the live ID.
- Public settings currently have no configured support, refund, privacy, terms, or status URLs. Do not invent legal policy or claim operational metrics.
- The public model-plaza API is `/api/v1/model-plaza`; the public settings API is `/api/v1/settings/public`.

## Goal

Implement the highest-value, low-risk online conversion/trust improvements in the source code, without changing billing calculations, model availability, upstream accounts, database schema, payment behavior, or deployment configuration.

## Required work

1. Inspect the current frontend/source structure and existing localization/content conventions before editing.
2. Fix the malformed Anthropic curl example in `public/docs/quickstart.html` (or its source, if generated) and make both curl examples copy-paste-safe using an environment-variable placeholder such as `$TASKSETLY_API_KEY`, without exposing any real secret. Keep the model placeholder explicit and add a concise instruction to copy the model ID from the live console/model plaza.
3. Improve the public landing/home copy in the correct source/config path, using verified capabilities only:
   - API access for AI coding tools / compatible clients.
   - Anthropic Messages and OpenAI Chat Completions examples are documented.
   - The console can show calls, cost and quota after a successful request.
   - Do not claim official upstream status, guaranteed stability, universal model coverage, guaranteed savings, or specific refund terms.
   - Add clear CTAs to the model plaza and quickstart/docs if the existing design supports them.
   - Preserve existing i18n and provide Chinese/English strings where the project convention supports both.
4. Improve model-plaza labeling/presentation only if the existing source makes this safe and localizable. Prefer neutral explanatory UI text that says group names are internal channel classifications and do not by themselves guarantee upstream identity, output quality, or continuous availability. Do not silently rename live database groups or alter pricing.
5. Add or improve navigation links to existing legal/support/status pages only when the repository already contains those pages or configurable routes. Do not fabricate legal policy text. If links cannot be safely wired without admin-configured URLs, document the gap rather than inventing it.
6. Add or update focused tests only if they do not require compilation/build; follow repository rules. Do not run compilation, build, or test commands that compile the project unless explicitly requested by the owner. Run `git diff --check` and static checks that do not compile.
7. Review the final diff for secrets, unsupported claims, unrelated changes, generated artifacts, and accidental billing/model changes.
8. Create a local git commit containing only this task's changes. Do not push.

## Acceptance criteria

- Quickstart Anthropic command has balanced quoting and no literal credential; OpenAI example is also safe to copy with an environment variable.
- Landing copy states what Tasksetly is and what the first-success flow is, without unsupported guarantees.
- Any new explanatory strings are localized according to existing conventions.
- No billing formulas, model groups, upstream account data, or payment behavior are changed.
- `git diff --check` passes.
- Codex reports changed files, static checks, skipped compile/test commands, and commit hash.
