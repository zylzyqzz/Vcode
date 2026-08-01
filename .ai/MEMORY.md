# Project Memory (verified facts)

- The product direction is terminal CLI first.
- Desktop is compatibility-only and is not part of the primary release path.
- DeepSeek remains the main model direction; Provider stays extensible.
- The CLI has a compact dark-gold activity UI with collapsed reasoning/tool output and `Ctrl+O` expansion.
- The agent runtime already has Coordinator, `task`, `parallel_tasks`, session persistence, checkpoint/rewind, compaction, MCP, Skills, goals, and project verification.
- `vcode task` is the durable task graph management entry point.
- Project task state is stored under `.vcode/tasks/`.
- New architecture work must preserve existing session, MCP, Skills, sandbox, and verification behavior.

## Facts still requiring product confirmation

- Exact global task index location and cross-project synchronization policy.
- Whether future worktree merging should create commits automatically.
- Remote execution and distributed worker support.
