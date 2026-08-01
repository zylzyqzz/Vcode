# 002 — Long-Task Runtime Foundation

## Goal

Move Vcode from model-prompt-only orchestration toward a durable, role-aware, recoverable multi-agent development runtime.

## Implemented

- Added role configuration under `agent.roles` with legacy planner/subagent fallback.
- Added task scheduler with dependency batches, bounded parallelism, lifecycle events, retry limits, cancellation, and final task outcomes.
- Added `vcode task run` to execute ready nodes through the existing Agent runtime and project verifier.
- Added Git worktree lifecycle management for Build nodes with traversal protection.
- Added global task index and `vcode task global` for cross-project task discovery.
- Added detailed task inspection, JSON output, lifecycle logs, retry/pause/resume/cancel commands.
- Added adversarial tests for concurrency limits, cancellation, verification outcomes, role routing, persistence, and Windows worktrees.

## Verification

- Passed: `go test ./internal/taskgraph ./internal/worktree`
- Passed: `go test ./internal/agent`
- Passed: focused `internal/config`, `internal/boot`, and `internal/cli` suites
- Passed: native CLI build and smoke tests
- Known legacy failure: two existing `internal/control` YOLO/plan approval assertions remain red and are unrelated to taskgraph code.

## Remaining work

- Automatic worktree merge/conflict-resolution nodes.
- Dedicated task progress events in the interactive TUI.
- Distributed/remote workers and richer extension SDK.
