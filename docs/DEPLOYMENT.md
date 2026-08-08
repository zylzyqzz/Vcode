# Vcode Deployment

## Verified current cloud runtime

- Host service: `vcode.service` managed by systemd.
- Launch script: `/opt/vcode/bin/vcode-start.sh`.
- Runtime bind: `127.0.0.1:18878`.
- Public edge: Nginx TLS virtual host for `v.aimj.xin` proxies to that loopback port.
- Runtime authentication: password mode; an unauthenticated local or external status request is expected to return HTTP 401.
- Artifact platform: Linux x86_64. On 2026-08-08 a Windows executable was found at the Linux service path and was replaced by a verified Linux x86_64 binary; the prior file was retained as a rollback artifact.

## Deployment controls

1. The runtime is replaced only by an immutable Linux x86_64 artifact with a SHA-256 checksum.
2. `.github/workflows/deploy-vcode-prod.yml` is the manual production deployment path after `master` protection is enabled.
3. Nginx validation and a loopback/public authentication smoke test are mandatory before activation.
4. The service preserves the last three verified artifacts and rolls back when restart or health validation fails.

## Required deployment contract

Before the next production deployment, establish and document:

1. a single protected release branch and matching workflow triggers;
2. immutable commit/tag, Linux architecture-specific artifact and SHA-256/provenance;
3. staged upload, pre-restart artifact verification, retained prior artifact and tested rollback;
4. `nginx -t` before every reload and authenticated end-to-end smoke test after restart;
5. monitoring that distinguishes a running process, reachable TLS edge, authentication response and usable agent service;
6. tested backup/restore ownership for runtime configuration, sessions/workspaces and every independent Worker/D1 database.

Changing this deployment architecture is L3 and needs explicit approval.
