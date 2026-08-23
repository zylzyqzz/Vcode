<p align="center">
  <img src="docs/logo.svg" alt="Vcode" width="640"/>
</p>

<p align="center">
  <strong>English</strong>
  &nbsp;·&nbsp;
  <a href="./README.zh-CN.md">简体中文</a>
  &nbsp;·&nbsp;
  <a href="./docs/GUIDE.md">Guide</a>
  &nbsp;·&nbsp;
  <a href="./docs/SPEC.md">Spec</a>
</p>

<p align="center">A DeepSeek-native AI coding agent for your terminal.</p>
<p align="center">A config- and plugin-driven harness — a single static Go binary, tuned around DeepSeek's prefix cache so token costs stay low across long sessions.</p>

<br/>

## Features

- **Config-driven.** Providers, the agent, enabled tools, and plugins are all declared in `Vcode.toml`. No hardcoded models.
- **Multi-model & composable.** DeepSeek ships as a preset; any OpenAI-compatible endpoint is a config entry, not new code. Optionally run two models together (executor + planner) in separate, cache-stable sessions.
- **Plugin-driven.** External tools run as subprocesses over stdio JSON-RPC (MCP-compatible). Built-in tools self-register at compile time.
- **Cache-aware context maintenance.** Startup injects a small stable environment summary, stale tool output is snipped/pruned before summary compaction, and the built-in tool schema contract is documented for regression review.
- **Zero-friction distribution.** `CGO_ENABLED=0` single binary; cross-compile to six targets with one command. Dependencies are vendored and the binary is self-contained — no runtime install needed.

## Install

```sh
npm i -g Vcode                  # any OS; pulls the prebuilt native binary
brew install esengine/Vcode/Vcode   # macOS
```

Prebuilt archives (`darwin|linux|windows × amd64|arm64`) and `SHA256SUMS` are on every [GitHub release](https://github.com/zylzyqzz/Vcode-go/releases).

### Code signing

Windows builds are code-signed with a free certificate provided by the [SignPath Foundation](https://signpath.org/), with signing through [SignPath.io](https://signpath.io/).

### Build from source

```sh
make build      # -> bin/Vcode(.exe)
make cross      # -> dist/ (darwin|linux|windows × amd64|arm64)
```

## Quick start

```sh
Vcode setup                      # config wizard → ./Vcode.toml
export DEEPSEEK_API_KEY=sk-...      # or let setup save it to Vcode home .env
Vcode                            # then run /init to generate AGENTS.md (project memory)
Vcode run "implement the TODOs in main.go"
Vcode run --model deepseek-pro "add unit tests for this function"
echo "explain this code" | Vcode run
```

## Configuration

A minimal `Vcode.toml` — one provider and a default model — is enough to start:

```toml
default_model = "deepseek-flash"

[[providers]]
name        = "deepseek-flash"
kind        = "openai"
base_url    = "https://api.deepseek.com"
model       = "deepseek-v4-flash"
api_key_env = "DEEPSEEK_API_KEY"
```

Resolution order is **flag > `./Vcode.toml` > the user config file > built-in defaults**; starting with **Vcode v1.8.1**, the user file lives at `~/.Vcode/config.toml` on macOS/Linux and `%AppData%\Vcode\config.toml` on Windows. See **[Configuration paths](./docs/CONFIG_PATHS.md)** for migration details and the full `config.toml` / `.env` structure. Provider entries name secrets with `api_key_env`; the secret values themselves live in Vcode's global `<Vcode home>/.env`, shared by CLI and serve. Permissions, the sandbox, plugins (MCP), slash commands, `@` references, and two-model setup are all in the **[Guide](./docs/GUIDE.md)**.

## Documentation

- **[Guide](./docs/GUIDE.md)** — configuration, permissions & sandbox, plugins (MCP), slash commands, `@` references, two-model collaboration.
- **[Bot guide](./docs/BOT_GUIDE.md)** — connect Feishu, Lark, and WeChat bots from the CLI, then use approvals, YOLO, and commands from IM.
- **[Spec](./docs/SPEC.md)** — engineering contract: architecture, registries, data types, and roadmap.
- **[Tool contract](./docs/TOOL_CONTRACT.md)** — provider-visible built-in tool names, read-only flags, and schema snapshot guard.
- **[Checkpoints & rewind](./docs/CHECKPOINTS.md)** — the snapshot-based edit safety net (Esc-Esc / `/rewind`).

<br/>

## License

MIT — see [LICENSE](./LICENSE)