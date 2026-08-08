# Vcode Database Boundaries

The core Go CLI, desktop app and hosted `serve` runtime do not declare a repository-managed SQL database. Their sessions and configuration are filesystem/config based; storage behavior must be inspected by the target subsystem before changing it.

Cloudflare Workers own the repository-managed databases:

| Service | Database | Source of truth |
| --- | --- | --- |
| Accounts | Cloudflare D1 | `workers/accounts/migrations/` and `wrangler.toml` |
| Crash report / registry | Cloudflare D1 | `workers/crash-report/registry-schema.sql` and `wrangler.toml` |
| Forum | Cloudflare D1 | `workers/forum/schema.sql`, seed/migration files and `wrangler.toml` |

Any D1 schema or migration change is L3: it requires an approved migration/rollback plan, backup/restore evidence, and deployment sequencing. The live backup/restore posture of these D1 databases is **Pending confirmation**.
