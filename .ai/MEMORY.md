# Vcode Long-Term Memory

- Vcode prioritizes a static, cross-platform Go binary, config-driven providers/plugins, and cache-stable long agent sessions. Source: `README.md`, `Makefile`, `.goreleaser.yaml`.
- The CLI, desktop app and hosted `serve` runtime share core Go packages but are separate build/deployment surfaces. Source: root and `desktop/` Go modules, `internal/serve`.
- Agent execution authority is intentionally powerful in Build mode. Production safety therefore depends on strong identity and infrastructure isolation rather than UI mode labels alone. Source: user-confirmed product direction and the deployed service configuration.
