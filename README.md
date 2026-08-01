<p align="center">
  <img src="docs/logo.svg" alt="Vcode" width="320"/>
</p>

<p align="center">
  <strong>Terminal-first coding agent for Windows, Linux, and macOS.</strong>
</p>

<p align="center">
  <a href="./README.zh-CN.md">绠€浣撲腑鏂?/a> 路
  <a href="./docs/GUIDE.md">Guide</a> 路
  <a href="./docs/SPEC.md">Architecture</a>
</p>

Vcode is a fast, configurable AI coding agent that lives in your terminal. It combines an agent loop, file and shell tools, MCP, Skills, session recovery, and project verification in one Go binary.

The default experience is deliberately compact: the terminal shows the current activity, a short tool summary, the final answer, and a small status line. Detailed reasoning and tool output remain available when you expand them with `Ctrl+O`.

## What Vcode does

- **Builds and edits projects** with read, search, write, patch, and shell tools.
- **Works well with DeepSeek** and keeps the provider layer open to other OpenAI-compatible APIs.
- **Understands long sessions** with checkpoints, rewind, fork, context compaction, and cache-aware prompts.
- **Extends through MCP and Skills** without making optional integrations part of startup cost.
- **Verifies completed work** with project-aware checks for Go, Node.js, Python, Rust, and configured project commands.
- **Makes safety visible** with permission prompts, path restrictions, and explicit Windows sandbox fallback states.

## Install

Download a release binary for your platform, or build from source with Go 1.24+:

```sh
make build
./bin/vcode version
```

Cross-platform release artifacts can be built with:

```sh
make cross
```

Vcode targets Windows, Linux, and macOS on amd64 and arm64. The desktop application is not part of the CLI product path.

## Quick start with DeepSeek

Create a project-level `vcode.toml`:

```toml
default_model = "deepseek-v4-flash"

[[providers]]
name = "deepseek"
kind = "openai"
base_url = "https://api.deepseek.com"
model = "deepseek-v4-flash"
api_key_env = "DEEPSEEK_API_KEY"

[sandbox]
bash = "auto"
```

Set the API key in your environment:

```powershell
$env:DEEPSEEK_API_KEY = "sk-..."
vcode
```

```sh
export DEEPSEEK_API_KEY=sk-...
vcode
```

For a guided setup, run:

```sh
vcode setup
```

## Common commands

```text
vcode                         Start an interactive session
vcode run "fix the failing tests"  Run one task and verify the project
vcode review                  Review the current worktree
vcode doctor                  Diagnose configuration, model, tools, and safety
vcode doctor --json           Print machine-readable diagnostics
vcode setup                   Create or migrate local configuration
vcode task list                List durable project tasks
vcode task list --json         Emit task state for scripts and CI
vcode task global              List tasks across projects
vcode task global --json       Emit the global task index as JSON
vcode task show <id>           Show task and node state (`--json` supported)
vcode task logs <id>           Show task lifecycle events (`--json` supported)
vcode task plan <id>           Generate a Chinese read-only execution plan
vcode task approve <id>        Approve the plan before any write-capable node
vcode task resume <id>         Recover interrupted work
vcode task retry <id> <node>   Retry one failed node
vcode task run <id>             Execute the task graph through Vcode agents
vcode task run <id> --no-verify Run explicitly without project checks (`UNVERIFIED`)
vcode task merge <id> [node]    Integrate committed node work into the project
```

Inside an interactive session:

- `Ctrl+O` expands or collapses the latest reasoning or tool output.
- `Ctrl+B` remains available for shell output compatibility.
- `/plan` starts a Chinese, read-only planning flow and waits for approval.
- `/rewind`, `/resume`, and `/fork` manage recoverable session history.

## Safety model

`[sandbox].bash` supports three modes:

- `auto`: use an OS sandbox when available; otherwise use path constraints, dangerous-command blocking, and approval prompts.
- `enforce`: refuse Bash execution when an OS-level sandbox is unavailable.
- `off`: disable sandboxing while retaining permission prompts and working-directory warnings.

On Windows, `auto` never pretends that a fallback policy is an OS-level jail. Use `vcode doctor --json` to inspect the effective mode and the reason for any downgrade.

## Configuration

Configuration is resolved in this order:

1. command-line flags;
2. project-level `./vcode.toml`;
3. user-level configuration;
4. built-in defaults.

See the [CLI guide](./docs/GUIDE.md) for providers, permissions, MCP, Skills, sessions, and configuration paths. Secrets should be supplied through environment variables or the Vcode user `.env` file; do not commit API keys.

Long-running work is stored under `.vcode/tasks/`. Task records preserve node dependencies, lifecycle events, retry state, artifacts, and verification results so another Vcode or AI agent can continue the project without reconstructing the whole task from chat history.

Role-specific model routing is available through `agent.roles`:

```toml
[agent.roles.plan]
model = "deepseek-v4-flash"
mode = "read_only"

[agent.roles.build]
model = "deepseek-v4-flash"
mode = "autonomous"
max_steps = 80
tools = ["read_file", "search", "write_file", "patch", "bash"]
```

For long-running development, create a durable task, inspect its plan, then run
the graph. Build nodes execute in isolated Git worktrees and record commits;
integration is explicit so a conflict blocks the task instead of silently
overwriting the main worktree:

```sh
vcode task create "瀹炵幇骞堕獙璇佺敤鎴疯璇佹ā鍧?
vcode task show <id> --json
vcode task run <id>
vcode task merge <id>
```

Each node records changed files, artifacts, retries, and verification evidence.
The final task outcome is `VERIFIED`, `PARTIAL`, or `UNVERIFIED`; a missing or
failed check is never presented as a successful completion.

## Development

```sh
go test ./internal/config ./internal/sandbox ./internal/doctor ./internal/verify ./internal/planmode
go test ./internal/cli -run 'Test(CtrlO|ReasoningSummary|Tool|Chooser)' -count=1
go build ./cmd/vcode
```

Vcode is released under the MIT License. See [LICENSE](./LICENSE).
