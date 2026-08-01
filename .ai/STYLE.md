# Code and Documentation Style

- Follow existing Go package boundaries and keep reusable logic outside CLI rendering code.
- Prefer typed events and structured results over parsing display strings.
- Keep task, node, agent, and tool identifiers stable across persistence and resume.
- Use explicit status names and error messages that tell the operator what can be retried.
- Keep terminal output compact by default; details must remain available through expansion or task logs.
- Documentation must distinguish verified facts from proposed improvements.
- Use `gofmt`; add focused tests beside new behavior.
