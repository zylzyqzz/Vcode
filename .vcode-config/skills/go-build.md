---
name: go-build
description: Go build, test, and lint workflow for this Vcode project
---

When working with Go code in this Vcode project:

1. **Build**: `go build ./cmd/Vcode/` — outputs to current dir
   - Full build: `go build -o bin/Vcode.exe ./cmd/Vcode`
   - Cross-compile: `GOOS=linux GOARCH=amd64 go build -o bin/Vcode-linux ./cmd/Vcode`

2. **Test**: `go test ./internal/...` — all packages
   - Single package: `go test ./internal/agent/...`
   - With race: `go test -race ./internal/...`

3. **Lint**: `golangci-lint run` — project has `.golangci.yml`
   - Or `go vet ./...`

4. **Check**: `go mod tidy` after dependency changes

5. Key build constraints: `CGO_ENABLED=0` for release builds
   Windows builds are code-signed via SignPath Foundation