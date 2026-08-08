# Vcode API Boundaries

## Hosted runtime

`internal/serve/serve.go` registers the browser-facing session endpoints. Verified categories include:

- session UI and state: `/`, `/events`, `/history`, `/context`, `/status`, `/sessions`, `/skills`, `/todos`;
- agent actions: `/submit`, `/cancel`, `/approve`, `/plan`, `/compact`, `/new`, `/rewind`, `/fork`, `/summarize`, `/answer`, `/resume`, `/forget`;
- mode/config actions: `/tool-approval-mode`, `/auto-approve-tools`, `/bypass`, `/goal`.

The service supports `none`, `token` and `password` auth modes. Production must use password or token auth behind TLS; the verified cloud instance uses password auth and loopback binding behind Nginx.

Endpoint and configuration compatibility is a public contract. New API behavior or any change to endpoint semantics requires focused tests and release notes; removals/renames are L3.

## Cloudflare APIs

The accounts Worker documents account, session and device-login routes in `workers/accounts/README.md`. Crash-report and forum APIs are independently deployed. Read the target Worker routes, schema and tests before changing an API; this file deliberately does not duplicate unverified endpoint details.
