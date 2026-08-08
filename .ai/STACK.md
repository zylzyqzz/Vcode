# Vcode Stack

- **Core:** Go 1.25 module with Go 1.26.4 toolchain; static `CGO_ENABLED=0` builds. Source: `go.mod`, `Makefile`.
- **CLI/TUI:** Charm Bubble Tea/Bubbles/Lipgloss; TOML configuration; built-in and MCP-compatible subprocess plugins. Source: `go.mod`, `README.md`.
- **Desktop host:** Go/Wails v2.12.0; system integrations include systray, updater and platform-specific files. Source: `desktop/go.mod`, `desktop/`.
- **Desktop UI:** React 19, TypeScript, Vite, pnpm, Mermaid and KaTeX. Source: `desktop/frontend/package.json`.
- **Hosted runtime:** Go HTTP service with password/token/none auth modes, intended for reverse-proxy deployment. Source: `internal/serve`, `vcode.example.toml`.
- **Cloud services:** TypeScript Cloudflare Workers (Hono, Zod, Wrangler) with Cloudflare D1 databases. Source: `workers/*/package.json`, `workers/*/wrangler.toml`.
- **Release tooling:** GoReleaser, GitHub Actions, npm package wrapper. Source: `.goreleaser.yaml`, `.github/workflows/`, `npm/vcode/`.
