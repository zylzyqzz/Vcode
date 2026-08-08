# Vcode Project Rules

## Scope and context

- Work only within the requested scope. Record unrelated findings in `tasks/backlog.md` as **AI suggestion, pending confirmation**.
- Start with the project map, Git delta and target module. Do not routinely rescan the repository or reread unchanged documentation.
- Code, executable configuration, tested behavior, and live runtime evidence outrank project documentation. Mark unresolved facts as `Pending confirmation`.

## Safety boundaries

- Do not overwrite unknown changes, reset, force-push, delete production data, or remove existing features without explicit authorization.
- L3 changes require an impact plan, rollback plan, verification plan, and explicit user approval.
- Treat configuration that enables agent execution, remote shell access, plugins, hooks, web serving, authentication, or credential storage as security-sensitive.
- Do not commit API keys, passwords, tokens, private keys, session material, or production environment files.

## Contracts and deployment

- Do not introduce breaking HTTP, CLI, configuration, plugin, or database contracts without confirmation and migration notes.
- Any production deployment must have an identified version/commit, verified artifact platform, preserved rollback artifact, service health check, and recorded result.
- Deployment documentation must distinguish verified live facts from intended workflow.

## Documentation and handoff

- Update only affected documents. Keep `STATE.md` short and factual.
- Effective work must leave a concise report under `.ai/reports/YYYY-MM/` and a recoverable `tasks/active.md` state.
- Facts require a code, configuration, Git, runtime, or user-confirmed source. Suggestions do not belong in factual documents.
