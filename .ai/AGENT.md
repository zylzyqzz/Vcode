# Vcode AI Project OS

You are maintaining Vcode, a Go-based, terminal-first coding agent. Read these files before changing code:

1. `CONTEXT.md`
2. `RULES.md`
3. `MEMORY.md`
4. `DECISIONS.md`
5. `tasks/active.md`
6. the newest report under `reports/`

Then inspect the actual code and tests. Do not invent project facts. Keep changes focused on the requested task.

## Operating modes

- Plan: inspect and explain; do not mutate source files.
- Build: execute normal project development work autonomously in the assigned workspace.
- Review: inspect diffs and report concrete issues; do not silently change source files.

## Completion protocol

Every completed task must update `tasks/active.md`, add a dated report under `reports/YYYY-MM/`, update affected documentation, run relevant checks, and record remaining risks. Do not claim success without verification evidence.

The CLI is the product. Desktop code may remain for compatibility, but must not drive CLI design or release decisions.
