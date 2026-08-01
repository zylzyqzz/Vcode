# Standard AI Development Workflow

1. Read `.ai/AGENT.md`, `.ai/CONTEXT.md`, `.ai/RULES.md`, `.ai/MEMORY.md`, `.ai/DECISIONS.md`, active tasks, and the newest report.
2. Inspect the relevant packages, configuration, tests, and git history.
3. State the target behavior, affected modules, risks, and verification commands.
4. Make the smallest coherent change. Keep public formats backward compatible.
5. Run focused tests first, then the broader package checks that fit the change.
6. Update active tasks, bug lists, affected docs, and a report under `.ai/reports/YYYY-MM/`.
7. Complete `.ai/CHECKLIST.md` and summarize evidence and remaining issues.

For long tasks, create or update a durable task graph under `.vcode/tasks/` before starting multi-agent execution.
