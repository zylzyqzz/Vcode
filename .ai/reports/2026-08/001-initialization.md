# 001 — AI Project OS Initialization

## Goal

Make Vcode understandable and maintainable by future AI agents while establishing the first durable long-task state boundary.

## Changes

- Added `.ai` project context, rules, workflow, memory, decisions, commands, checklist, tasks, bug lists, prompts, and this report.
- Added `internal/taskgraph` with JSON persistence, dependency validation, ready-node calculation, event append, and interrupted-node recovery.
- Added `vcode task list`, `vcode task create`, `vcode task show`, `vcode task resume`, `vcode task retry`, `vcode task pause`, and `vcode task cancel`.
- Added architecture, persistence, API, deployment, and documentation index pages.

## Verification

- `go test ./internal/taskgraph`
- targeted `internal/cli` tests

## Remaining work

Connect the persisted graph to Coordinator dispatch, role-specific agent profiles, worktree isolation, retries, resume, and task-aware TUI progress.
