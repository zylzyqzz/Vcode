# Vcode Context

Vcode is a config- and plugin-driven AI coding agent distributed as a static Go CLI, a Wails desktop app, and a self-hosted browser runtime. The public README describes DeepSeek as the bundled preset while allowing OpenAI-compatible and Anthropic providers.

The current user priority is stable self-hosted use, including a cloud runtime accessed from mobile. The immediate project stage is production-readiness remediation planning after a two-mode Build/Plan product change.

Primary constraints:

- Build mode is intended to execute with full user-authorized access; therefore server identity, authentication, network boundary and operational recovery are production-critical.
- Secrets must remain in environment/config storage outside Git.
- The repository includes independently deployable Cloudflare account, crash-report and forum services; they must not be treated as part of the single server binary deployment.
