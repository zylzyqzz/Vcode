# Verified Commands

Run from repository root unless noted.

```sh
go test ./...                 # root Go tests
go test -race ./...           # root Go concurrency checks
go vet ./...                  # root static analysis
make build                    # static CLI + example plugin for host platform
make cross                    # six CLI artifacts under dist/
```

Run from `desktop/`:

```sh
go test ./...
go vet ./...
```

Run from `desktop/frontend/`:

```sh
pnpm test
pnpm build
pnpm audit --prod --json
```

Verified live-server operations (root access required; deployment is L3):

```sh
systemctl status vcode.service --no-pager
systemctl restart vcode.service
curl -i http://127.0.0.1:18878/status # unauthenticated response is expected to be 401
```

Do not treat `docs/RELEASING.md` as an executable production runbook until its `main-v2` branch assumptions are reconciled with the verified `master` default branch.
