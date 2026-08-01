# CLI Deployment

Vcode is distributed as a platform-specific CLI binary for Windows, Linux, and macOS. Build locally with `make build` and cross-build with `make cross`.

Project task state belongs to the project and should be preserved with the workspace when long-running work must resume. API keys stay in environment variables or the user environment file and must never be committed.

Use `vcode doctor --json` to inspect provider, tools, network, session, verification, and effective sandbox state before unattended work.
