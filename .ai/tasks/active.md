# Active Tasks

- [x] Durable task graph with isolated build worktrees, commit/integrate flow, retry, recovery, and verification outcomes.
- [x] Role-specific model and tool boundaries for long-task agents.
- [ ] Compact TUI task phase summaries and cross-platform long-task smoke tests.

## AI Project OS initialization and durable task graph

- Status: in progress
- Goal: make Vcode maintainable by any AI and support recoverable long-running multi-agent development.
- Completed: project facts, operating rules, task graph store, dependency readiness, interrupted-node recovery, lifecycle events, task management commands, and role-specific model routing with legacy fallback.
- Completed: task graph execution through the Agent runtime and isolated worktree lifecycle primitives.
- Completed: task graph execution, role routing, bounded scheduler, verification outcomes, worktree provisioning, global index, task logs and JSON inspection.
- Next: merge/conflict-resolution nodes, task-aware interactive TUI progress, and distributed worker extensions.
- Verification: `go test ./internal/taskgraph`; targeted CLI tests; `go build ./cmd/vcode`.
