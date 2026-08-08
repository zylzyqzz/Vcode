# Decisions

## Decision: Adopt AI Project OS v2.0 governance adapted for Vcode

- **Status:** accepted
- **Date:** 2026-08-08
- **Reason:** User requested durable, low-context project rules before production review.
- **Impact:** `.ai/` becomes the entrypoint for future AI work; project-specific documents override generic templates where the template does not match Vcode.
- **Not selected:** copying empty generic database/API/deployment templates without verified Vcode facts.
- **Future adjustment:** allowed when the repository topology or operations model changes.
- **Evidence:** user-provided `AI-PROJECT-OS-v2.0(1).md`.

## Decision: Deploy Vcode cloud runtime as a Linux x86_64 binary managed by systemd

- **Status:** existing implementation observed; origin not confirmed
- **Date observed:** 2026-08-08
- **Reason:** Pending confirmation
- **Impact:** `/opt/vcode/bin/vcode` is launched by `vcode.service`; deployment must build the correct Linux artifact and retain a rollback copy.
- **Not selected:** Pending confirmation
- **Future adjustment:** requires L3 approval because it changes core deployment architecture.
- **Evidence:** verified live systemd service and filesystem inspection.
