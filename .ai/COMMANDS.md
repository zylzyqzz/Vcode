# Project Commands

```text
vcode                         Start interactive CLI
vcode run "<task>"            Execute one task and verify the project
vcode task list                List durable project tasks
vcode task create <goal>      Create a durable task record
vcode task show <task-id>     Show task and node state
vcode task resume <task-id>   Recover interrupted nodes and continue scheduling
vcode task retry <task-id> <node-id>  Reset one node for another attempt
vcode task pause <task-id>    Pause a task
vcode task cancel <task-id>   Cancel a task
vcode doctor                  Diagnose configuration and safety
vcode doctor --json           Machine-readable diagnostics
```

Interactive commands include `/plan`, `/build`, `/plan-exec`, `/review`, `/rewind`, `/resume`, `/fork`, and `/compact`.
