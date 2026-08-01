# Current Project Context

Vcode is a cross-platform Go CLI coding agent. DeepSeek is the primary provider direction, while the provider interface remains OpenAI-compatible and configurable.

Core runtime areas:

- `internal/agent`: Agent Loop, sessions, compaction, subagents, Coordinator, parallel research.
- `internal/control`: turn orchestration, goals, plans, approvals, todo state, verification flow.
- `internal/boot`: runtime assembly, providers, tools, MCP, Skills, sandbox, and role-specific agents.
- `internal/cli`: terminal commands and Bubble Tea TUI.
- `internal/config`: project/user configuration and migration.
- `internal/taskgraph`: durable project task graph state.
- `internal/verify`: project-aware completion checks.

The primary product goal is reliable long-running development with multiple specialized agents. Short tasks should remain fast and use the normal Agent Loop. Complex tasks should use durable task state, role-specific workers, isolated workspaces, verification, and resumability.

The repository currently supports Windows, Linux, and macOS CLI builds. Windows sandbox fallback must be explicit and must not be represented as OS-level isolation.
