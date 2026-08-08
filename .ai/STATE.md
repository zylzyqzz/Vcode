# Vcode State

- **Mode:** daily development; production-readiness audit complete.
- **Active task:** awaiting approval for P0 production remediation.
- **Branch:** `agent/build-plan-modes`.
- **HEAD:** `4793c5128f25bc9c2f5cceecf0a16be7129055db`.
- **last_synced_commit:** `4793c5128f25bc9c2f5cceecf0a16be7129055db` (GitHub branch pushed; PR #18 is draft).
- **Working tree at initialization:** clean before AI Project OS documents were created.
- **Document status:** initial project map and verified operating facts are available; audit report `reports/2026-08/001-production-readiness-audit.md` is the current production reference.
- **Known high-risk facts:** Nginx configuration cannot pass `nginx -t`; GitHub default branch bypasses core CI/CodeQL due to `main-v2` mismatch and has no protection; root/password SSH is publicly enabled without active brute-force protection; root-capable Vcode shares a host with unrelated public services.
- **Last updated:** 2026-08-08 America/Los_Angeles.
