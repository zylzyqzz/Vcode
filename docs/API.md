# Task Graph API

The `internal/taskgraph` package provides the first durable task API:

- `NewStore(projectRoot)` creates a project task store.
- `Store.Create` persists a task and its nodes.
- `Store.Get` and `Store.List` inspect tasks.
- `Store.Save` atomically persists changes.
- `Store.AppendEvent` records lifecycle events.
- `ReadyNodes` returns nodes whose dependencies succeeded.
- `Store.RecoverInterrupted` converts running nodes to resumable interrupted nodes.

The API is deliberately independent from the TUI. A scheduler can later consume ready nodes and dispatch role-specific Agent runners without changing the persistence contract.
