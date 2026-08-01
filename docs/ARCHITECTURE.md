# Vcode Architecture

Vcode is a Go terminal coding agent. The CLI assembles providers, tools, permissions, sandbox policy, MCP, Skills, sessions, and verification through `internal/boot`.

The runtime has two execution layers:

1. The Agent Loop (`internal/agent`) handles provider turns, tools, compaction, sessions, and subagents.
2. The task graph (`internal/taskgraph`) persists long-running goals, node dependencies, node roles, artifacts, verification, and lifecycle events under `.vcode/tasks/`.

The current task graph is intentionally independent from the CLI and Agent packages so TUI, ACP, and headless runners can share it. Coordinator and parallel subagents are the execution primitives; the next stage connects them to durable nodes.

Writable nodes use `internal/worktree` to create `.vcode/worktrees/<task-id>/<node-id>/` Git worktrees. The manager rejects traversal identifiers, records changed files, commits successful work, and uses Git's own worktree registry for create/list/remove lifecycle operations. `vcode task merge` cherry-picks a node commit into the project and aborts cleanly on conflicts.

`internal/taskgraph.Scheduler` is the durable orchestration boundary: it recovers interrupted nodes, respects dependencies, limits parallelism, retries failures, emits lifecycle events, and converges the task outcome from verification evidence. Role tool allowlists in `agent.roles.<role>.tools` narrow the executor surface for specialized agents without changing the shared MCP/Skills registry.

The default CLI plan starts two independent read-only scouts in parallel (code structure and test/verification coverage), feeds both reports into a Chinese planning role, then passes the persisted summaries into build and verification roles. This is the baseline multi-agent workflow; larger projects can edit the durable graph to add more bounded specialists.

Session transcripts remain the conversation source of truth. Task graphs remain the execution source of truth. A task result must include verification evidence before it can be reported as complete.
