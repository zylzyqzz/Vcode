# Persistence

Vcode does not require a database server. Sessions use JSONL files and sidecars. Long-running project tasks use JSON files under `.vcode/tasks/`.

Task files contain the goal, nodes, dependencies, status, attempts, artifacts, verification, and lifecycle events. Writes use a temporary file followed by rename so a process interruption does not intentionally replace a complete task file with a partial write.

The format is local and versionless in this first increment; future incompatible changes must add an explicit migration before changing existing fields.
