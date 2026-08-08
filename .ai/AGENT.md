# Vcode AI Agent Entry

Vcode uses AI Project OS v2.0 (Lean Context Protocol). The governing principle is: **minimal necessary context, maximum reliable execution**.

## Normal task entry

Read only these files first:

1. `.ai/STATE.md`
2. `.ai/tasks/active.md`
3. `.ai/PROJECT_MAP.md`
4. `.ai/RULES.md`

Then inspect `git status`, the current branch and HEAD, and the delta since `last_synced_commit`. Do not scan the whole repository unless the state documents are stale or the user explicitly requests initialization/re-inventory.

## Scope levels

- **L0:** single-file or tiny change — inspect target and direct tests.
- **L1:** one module — inspect that module, related types and tests.
- **L2:** cross-module — read relevant architecture/API/deployment documents and test integration points.
- **L3:** database migration, breaking API, authentication/authorization change, core deployment change, or broad refactor — produce a plan and wait for explicit user approval before implementation.

## On every effective code task

1. Preserve unknown local changes.
2. Review the diff and run proportionate verification.
3. Update only documentation affected by the change.
4. Update `tasks/active.md`, write a concise report, then update `STATE.md`.
5. Never place secrets in tracked files, reports, diffs, or chat output.

For the complete generic source policy, use the user-provided **AI Project OS v2.0** document; this repository adaptation is authoritative for Vcode-specific work.
