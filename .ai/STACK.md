# Verified Technology Stack

- Language: Go.
- CLI: Bubble Tea v2, Lip Gloss v2, terminal-aware rendering.
- Provider protocol: internal provider abstraction with OpenAI-compatible HTTP support.
- Configuration: TOML, project-level `vcode.toml`, user-level configuration, environment secrets.
- Persistence: JSONL sessions and JSON sidecars; task graph JSON under `.vcode/tasks/`.
- Extensions: MCP-compatible stdio plugins and project/global Skills.
- Verification: project-aware checks for Go, Node.js, Python, Rust, and configured commands.
- Platforms: Windows, Linux, macOS; amd64 and arm64 release targets.
- Build mode: `CGO_ENABLED=0` single CLI binary where supported by dependencies.
- Role routing: `agent.roles.<plan|explore|build|test|review>` with model, effort, mode, max_steps, and tool scope.
