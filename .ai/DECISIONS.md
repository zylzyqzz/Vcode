# Architecture Decisions

## Task graph is the durable long-task boundary

- Reason: goals and todo state are useful orchestration hints but are not enough to resume a multi-agent task after interruption.
- Alternatives not selected: keeping all orchestration inside model prompts; using only ephemeral in-memory state.
- Revisit: when a remote/distributed scheduler is introduced.

## Roles are first-class but Vcode-native

- Roles: Plan, Explore, Build, Test, Review.
- Reason: clear tool scope and model routing are needed for long coding tasks.
- Alternatives not selected: copying OpenCode configuration or embedding the Pi TypeScript runtime.
- Revisit: when external extension compatibility becomes a product requirement.

## Build is autonomous for normal project development

- Reason: long tasks lose most of their value if every ordinary edit or test requires interaction.
- Boundary: invalid paths, impossible tool arguments, and host-level safety failures remain hard failures; normal project development is not interrupted by approval prompts.
- Revisit: when remote execution or publishing workflows are added.
