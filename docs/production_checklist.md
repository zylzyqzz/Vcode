# Vcode Production Release Checklist

This checklist is the release gate for the protected `master` branch and the
public cloud runtime.

## Repository gate

- Required GitHub checks pass: root Go vet/test/race, desktop build/tests,
  frontend build and production dependency audit, CodeQL and lint.
- The release commit is on protected `master`; no direct push or force push was
  used.
- A Linux x86_64 static artifact and SHA-256 are produced from the exact commit.
- No unaccepted moderate-or-higher production dependency advisory remains.

## Server gate

- `nginx -t` succeeds before every reload.
- HTTPS certificate validity exceeds 14 days; `/status` returns the expected
  unauthenticated 401 at the loopback and public edge.
- `vcode-prod.service` is active; root Build authority is an explicit accepted
  operating risk on the shared host.
- Root/password SSH is disabled; key-only operator access and cloud-console
  recovery have been verified.
- Disk usage is below 85%; 75% usage is recorded for remediation.

## Release and recovery gate

- The deployment uses the immutable artifact workflow and preserves the three
  most recent verified releases.
- A restart/reconnect smoke test passes from the mobile browser.
- The daily encrypted backup has completed successfully, or the release is
  blocked until its external backup configuration is present.
- A rollback drill and restore drill have been run for the current release line.
