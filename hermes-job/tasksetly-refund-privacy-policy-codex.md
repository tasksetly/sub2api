# Tasksetly refund and privacy policy copy

## Owner-confirmed facts

The owner confirms the following exact product policies:

- Balance refunds are supported.
- The used portion of a balance is not refundable.
- Tasksetly will not retain user data.

Preserve the meaning of these statements exactly. Do not invent refund windows, processing times, fees, eligibility exceptions, payment-channel rules, legal guarantees, retention periods, deletion workflows, logging details, encryption claims, subprocessors, or jurisdiction-specific legal language.

## Task

Repository: `/home/debian/ai/sub2api-deploy/src/sub2api`
Read `AGENTS.md`, current Git status, and the existing legal-document/settings/localization conventions.

Implement the smallest safe source change that makes these owner-confirmed policies visible to unauthenticated users on the public site, preferably through existing legal-document routes/components and the existing Chinese/English localization convention. If the current architecture only displays admin-configured legal documents and cannot safely seed content from source, add clearly labeled localized policy copy in the existing public-facing help/legal surface without changing database schema, billing logic, refund APIs, payment behavior, or data-handling code.

Requirements:

1. Add a clearly labeled refund policy section: unused balance may be refunded; the already-used portion is not refundable.
2. Add a clearly labeled privacy statement: Tasksetly does not retain user data. Do not expand this into claims about transient processing, logs, backups, providers, or deletion unless already verified in the repository.
3. Keep Chinese and English copy aligned. Use the owner's facts, not marketing language.
4. Link the policy from the public landing/footer only where the existing route/configuration supports it. Do not fabricate missing legal pages or admin settings.
5. Add/update focused non-compiling checks only if appropriate. Follow `AGENTS.md`: do not run compilation, build, or compiling tests; run `git diff --check` and static checks.
6. Review for unsupported claims and secrets. Create a local commit containing only this task's source changes. Do not push.

Report changed files, checks, skipped checks, and commit hash. Leave unrelated untracked files untouched.
