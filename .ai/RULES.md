# Maintenance Rules

- Do not remove existing features without an explicit product decision.
- Do not change persisted session or task formats without a compatibility path.
- Do not claim a task is complete without test/build/verification evidence.
- Do not hide permission failures, tool errors, cancellation, or verification failures.
- Keep ordinary CLI tasks fast; optional planners, memory, MCP, and subagents should not add startup work unless enabled.
- Do not treat Windows fallback restrictions as an OS sandbox.
- Any cross-package architecture change must update `ARCHITECTURE.md`, `DECISIONS.md`, and a task report.
- Any public command or configuration change must update README and `COMMANDS.md`.
- Before committing, complete `CHECKLIST.md`.
