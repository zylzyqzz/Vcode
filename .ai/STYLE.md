# Vcode Style

- Go packages are organized by subsystem under `internal/`; entrypoints remain thin and wire registries through imports.
- Tests live beside Go code with `*_test.go`; desktop frontend tests are executable TypeScript files run sequentially by pnpm scripts.
- Configuration uses TOML with explanatory comments and secret environment-variable references rather than literal secret values.
- Existing modules contain historical product terminology and some documentation that no longer matches current behavior. Modify a module using its local conventions; do not perform broad cleanup without approval.
- User-visible English and Simplified Chinese documents commonly exist in parallel; update both only when the affected public behavior is documented in both.
