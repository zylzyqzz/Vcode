VERSION := $(shell git describe --tags --always 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
GOEXE := $(shell go env GOEXE)

.PHONY: build build-cli vet fmt fmt-check test test-cli test-task-runtime smoke-cli check hooks cross clean

build-cli:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/vcode$(GOEXE) ./cmd/vcode

build:
	$(MAKE) build-cli
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/vcode-plugin-example$(GOEXE) ./cmd/vcode-plugin-example

vet:
	go vet ./...

fmt:
	gofmt -w .

fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "These files are not gofmt-clean:"; gofmt -l .; exit 1)

test:
	go test ./...

test-cli: test

test-task-runtime:
	go test ./internal/taskgraph ./internal/worktree ./internal/verify ./internal/boot ./internal/cli -count=1

smoke-cli: build-cli
	bin/vcode$(GOEXE) version
	bin/vcode$(GOEXE) doctor --json >/dev/null

check: fmt-check vet test-cli

hooks:
	@git config core.hooksPath .githooks
	@echo "installed: core.hooksPath -> .githooks (pre-push runs go vet)"

cross:
	@mkdir -p dist
	@for p in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do \
		os=$${p%/*}; arch=$${p#*/}; ext=; [ $$os = windows ] && ext=.exe; \
		echo "build $$os/$$arch"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -ldflags "$(LDFLAGS)" -o dist/vcode-$$os-$$arch$$ext ./cmd/vcode; \
	done

clean:
	rm -rf bin dist
