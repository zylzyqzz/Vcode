# Vcode Architecture

Vcode is a Go terminal coding agent. The CLI assembles providers, tools, permissions, sandbox policy, MCP, Skills, sessions, and verification through `internal/boot`.

The runtime has two execution layers:

1. The Agent Loop (`internal/agent`) handles provider turns, tools, compaction, sessions, and subagents.
2. The task graph (`internal/taskgraph`) persists long-running goals, node dependencies, node roles, artifacts, verification, and lifecycle events under `.vcode/tasks/`.

The current task graph is intentionally independent from the CLI and Agent packages so TUI, ACP, and headless runners can share it. Coordinator and parallel subagents are the execution primitives; the next stage connects them to durable nodes.

Writable nodes use `internal/worktree` to create `.vcode/worktrees/<task-id>/<node-id>/` Git worktrees. The manager rejects traversal identifiers and uses Git's own worktree registry for create/list/remove lifecycle operations.

Session transcripts remain the conversation source of truth. Task graphs remain the execution source of truth. A task result must include verification evidence before it can be reported as complete.
