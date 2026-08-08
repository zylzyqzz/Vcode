# Production Readiness Audit

- **Task ID:** 2026-08-production-readiness-audit
- **Date:** 2026-08-08
- **Scope:** repository governance initialization, code/build controls, GitHub delivery controls, and read-only inspection of the live Vcode cloud runtime.
- **Conclusion:** **not production-ready until P0 items are remediated and verified.** The current service is running, but release integrity and host security are insufficient for a public, root-capable coding agent.

## Verified strengths

- Root Go module: `go vet ./...` and `go test -race ./...` completed without reported failures during this audit; the configured vulnerability scan completed without findings.
- The desktop frontend test suite had previously passed for the deployed code revision; a repeat run in this audit was stopped after the tool left child processes in the background. It must be run cleanly in CI before release.
- Hosted Vcode binds to `127.0.0.1`, uses password authentication, is systemd-managed and restarts automatically.
- TLS is valid for the public host at the time of inspection and the unauthenticated public status request returns 401.
- The root codebase has extensive Go tests and CI includes vet, build, test, race, lint, coverage and CodeQL definitions.

## P0 — production blockers

### 1. Nginx configuration is corrupt

- **Evidence:** `nginx -t` fails with `unknown directive` at `/etc/nginx/conf.d/vcode.conf:1`; inspection shows leading NUL bytes. HTTPS still responds only because Nginx is running an older in-memory configuration.
- **Impact:** proxy or certificate changes cannot be safely reloaded; a restart/reload may make the hosted runtime unavailable.
- **Required fix:** preserve the corrupt file, reconstruct a minimal known-good Vcode virtual host from the verified live/backup configuration, run `nginx -t`, reload, then prove HTTPS/authentication and WebSocket/SSE behavior. Add a pre-reload check to every deployment.
- **Rollback:** restore the exact prior valid configuration only after `nginx -t` passes.

### 2. Default branch bypasses core quality and security gates

- **Evidence:** GitHub default branch is `master`; CI, CodeQL, Pages, Workers and release documentation target `main-v2`. `master` branch protection API reports it is not protected. PR #18 received labels but no core CI checks.
- **Impact:** a normal merge to the default branch can bypass tests, race checks, CodeQL, site/Worker delivery expectations and review requirements.
- **Required fix:** choose one trunk (`master` or `main-v2`), retarget every workflow/document/release action, require pull requests plus successful required checks, and prevent direct pushes. This decision needs owner approval because it changes delivery governance.
- **Rollback:** retain the existing branch until the new protections and a canary run are verified.

### 3. Public root-password SSH is exposed without active brute-force protection

- **Evidence:** effective SSH config allows `PermitRootLogin yes` and `PasswordAuthentication yes`; fail2ban is inactive; host firewall policy accepts inbound traffic; the login banner reported a very high failed-login count.
- **Impact:** direct root password attacks threaten every workload, credentials and the root-capable Vcode agent.
- **Required fix:** create a named operator/deploy account with verified key-based access and recovery access first; then disable root/password SSH, restrict inbound SSH at the cloud security group/firewall, enable rate limiting/fail2ban, and document break-glass recovery.
- **Rollback:** keep verified console access and a second key-authenticated operator session before changing SSH.

### 4. Root-capable Vcode shares a host with unrelated public services

- **Evidence:** `vcode.service` runs as root with no `NoNewPrivileges`, `ProtectSystem` or `ProtectHome`; the server also runs Nginx, Docker services and unrelated applications.
- **Impact:** a Vcode session, plugin, prompt-injection path or dependency compromise has a direct route to the entire multi-service host. Build mode's intended full authority makes this architectural, not merely a systemd-hardening issue.
- **Required fix:** move Vcode to a dedicated VM/host where full Build authority is isolated, or approve a new privilege/sandbox model. Do not label the existing shared host as production-safe for public remote coding.
- **Rollback:** keep the existing service as a non-production/private instance until the isolated target passes a migration runbook.

## P1 — must complete before stable rollout

1. **Reproducible release/deploy:** build Linux artifacts in CI from protected commits, attach SHA-256/provenance, deploy atomically, retain one known-good release and verify rollback automatically. The Windows-binary incident proves manual artifact selection is currently unsafe.
2. **Monitoring and recovery:** add independent process, TLS edge, authenticated service and agent-synthetic checks; alert on certificate expiry, `nginx -t` failure, restart loops, disk space and backup failure. The certificate observed expires 2026-11-02; corrupted Nginx configuration can prevent successful renewal reload.
3. **Host lifecycle:** schedule the visible operating-system updates and a controlled reboot/health validation window. Confirm firewall policy and backup/restore coverage for Vcode runtime data.
4. **Frontend dependency remediation:** `pnpm audit --prod --json` found 2 low and 5 moderate production vulnerabilities through `mermaid@11.16.0` and `dompurify`. Upgrade to at least Mermaid 11.16.1 and DOMPurify 3.4.13, then rerun Mermaid sanitization tests and the full frontend suite.
5. **Desktop release gate:** make the desktop Go tests and frontend build reliably observable in CI. Earlier local full desktop testing had asset-specific failures/missing Windows ICO evidence; fix or explicitly gate platform release artifacts.
6. **Worker delivery:** Dependabot covers root, desktop and site packages but not the three Worker pnpm projects. Add them, and align Worker deployment triggers with the chosen protected trunk.

## P2 — governance and maintainability

1. Replace or archive stale product documents: `docs/production_checklist.md` describes unrelated v5 system concepts and is not an executable Vcode release gate.
2. Remove legacy Goal/YOLO/approval-mode terminology from public docs/config examples that conflict with the current Build/Plan product decision, after a documentation impact review.
3. Establish explicit SLOs: availability target, recovery-time/recovery-point targets, supported platform matrix, incident owner and release owner.
4. Add a production smoke-test specification for mobile browser behavior, reconnects, long-running agent tasks, reverse-proxy upgrades/SSE, and rollback.

## Recommended execution order

1. Repair and validate Nginx configuration without changing Vcode behavior.
2. Secure operator access and isolate the Vcode host/VM boundary.
3. Select/protect the trunk and make CI/CodeQL/release triggers match it.
4. Implement reproducible canary deployment and tested rollback.
5. Upgrade frontend dependencies and clear full desktop release gates.
6. Add monitoring, backups/restore drills and documented SLOs.

## Verification gate for calling the service production-ready

- `nginx -t` succeeds; TLS and authenticated interactive runtime smoke tests pass after a controlled reload.
- Root/password SSH is disabled after verified key-based operator and break-glass access.
- Vcode runs on an isolated production host/VM or an approved equivalent security boundary.
- Protected trunk requires the full root, desktop and frontend gates; CodeQL runs on it.
- A canary from the protected commit deploys a correct Linux artifact, passes health/synthetic checks, and rolls back successfully in a drill.
- Dependency audit has no unaccepted moderate-or-higher production issues.
- Backups and restores are tested for runtime configuration/data and each D1-backed service.
