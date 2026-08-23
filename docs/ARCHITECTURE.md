# Vcode Architecture

## Verified component model

```text
CLI / browser client (PWA)
        │
        ├── static Go CLI and agent core (`cmd/vcode`, `internal/`)
        └── hosted Go HTTP runtime (`internal/serve`)
                │
                ├── providers (OpenAI-compatible / Anthropic)
                ├── built-in tools, plugins and agent execution
                └── workspace and session state
```

The server-hosted runtime is a separate operational surface, not a replacement for the Cloudflare services. Its HTTP handler provides interactive session endpoints and an auth gate; it is currently deployed behind Nginx and bound to loopback.

## Trust boundaries

- Provider credentials are environment-resolved; they must not enter repository configuration.
- Build mode may execute tools with broad user-authorized authority. Treat every authenticated hosted session, plugin and workspace as a privileged control-plane input.
- Nginx/TLS and Vcode authentication are the public edge for the current cloud runtime. SSH/root access is a separate operator boundary.

## Production architecture decision needed

The verified host also runs unrelated public services and Docker workloads, while Vcode runs as root. A production hosted Build mode should either run in an isolated single-purpose VM/host or use a deliberately designed sandbox/privilege model. This is a required architectural decision before declaring the service production-ready.
