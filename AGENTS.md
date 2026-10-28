# Repository working rules

- When a requested feature or fix is complete, create a local Git commit for the changes made for that task. Do not push the commit to a remote unless explicitly asked.
- Stage only files changed for the current task. Never include unrelated pre-existing, untracked, or user-owned changes in the commit; leave those untouched.
- After writing or editing code, do not run compilation, build, or test commands that compile the project unless the user explicitly requests them. Prefer non-compiling checks such as `git diff --check` and static inspection, and report which checks were skipped.
