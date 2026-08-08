# Vcode Project Map

## Product surfaces

| Path | Responsibility | Primary entry/tests |
| --- | --- | --- |
| `cmd/vcode` | Static Go CLI and HTTP `serve` command | `cmd/vcode/main.go`; `internal/**/*_test.go` |
| `internal/` | Agent orchestration, tools, providers, config, CLI, HTTP serving, memory, sandbox and plugins | `internal/cli`, `internal/serve`, `internal/control`, `internal/tool` |
| `desktop/` | Wails desktop host, local runtime bridge, updater and OS integration | `desktop/main.go`, `desktop/app.go`, `desktop/**/*_test.go` |
| `desktop/frontend/` | React/Vite desktop UI | `src/`, `src/__tests__/`, `pnpm test` |
| `workers/accounts/` | Cloudflare Worker account/session API backed by D1 | `src/`, `migrations/`, `wrangler.toml` |
| `workers/crash-report/` | Crash reporting and registry Worker backed by D1 | `src/`, `wrangler.toml` |
| `workers/forum/` | Community forum Worker backed by D1 | `src/`, `schema.sql`, `wrangler.toml` |
| `site/` | Public site frontend | `package.json`, `src/` |
| `npm/vcode/` | npm distribution wrapper for native binaries | `package.json` |

## Key boundaries

- `cmd/vcode/main.go` wires providers and built-in tools; `internal/cli` owns commands and session execution.
- `internal/serve` exposes the browser-facing coding runtime. Its default bind/address, auth and proxy behavior are driven by config and CLI flags.
- `internal/config` owns TOML and environment resolution. `vcode.example.toml` is the verified public configuration reference; secrets are referenced by environment-variable name.
- `desktop/` is a separate Go module (`desktop/go.mod`) which replaces the root module locally. The desktop frontend is a separate pnpm project.
- The Workers are independent TypeScript/pnpm projects. Their D1 schemas and migrations are their database source of truth.

## Delivery and operations

- Root CI: `.github/workflows/ci.yml`; CodeQL: `.github/workflows/codeql.yml`; releases: `.github/workflows/release*.yml` and `.goreleaser.yaml`.
- Verified live server: systemd service `vcode.service`, launch script `/opt/vcode/bin/vcode-start.sh`, runtime address `127.0.0.1:18878`, service data/config under `/opt/vcode` and `/etc/vcode`.
- `docs/RELEASING.md` and many workflows name `main-v2`; GitHub default branch is verified as `master`. Treat all automatic delivery assumptions as stale until reconciled.

## Read first by task

- CLI/tool behavior: `internal/cli`, `internal/control`, `internal/tool`, then related tests.
- Browser runtime/auth: `internal/serve/auth.go`, `internal/serve/serve.go`, their tests, then `docs/API.md`.
- Desktop behavior: target file under `desktop/`, related frontend component, then relevant Go and TS tests.
- Cloud Worker/API/database: target `workers/<name>/src`, `wrangler.toml`, schema/migrations, and `docs/DATABASE.md`/`docs/API.md`.
- Deployment: `.ai/COMMANDS.md`, `docs/DEPLOYMENT.md`, systemd and proxy evidence; do not infer deployment from release docs alone.
