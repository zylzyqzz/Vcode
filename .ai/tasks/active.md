# Active Tasks

## AI Project OS initialization and durable task graph

- Status: in progress
- Goal: make Vcode maintainable by any AI and support recoverable long-running multi-agent development.
- Completed: project facts, operating rules, task graph store, dependency readiness, interrupted-node recovery, lifecycle events, and `vcode task` list/create/show/resume/retry/pause/cancel.
- Next: connect task graph execution to Coordinator, role routing, worktree lifecycle, and task-aware TUI.
- Verification: `go test ./internal/taskgraph`; targeted CLI tests; `go build ./cmd/vcode`.
