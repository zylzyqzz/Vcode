# Vcode 长任务与多智能体开发流程

Vcode 的长任务不是一段不可恢复的聊天记录，而是一份位于 `.vcode/tasks/` 的持久化任务图。每个节点有角色、依赖、重试次数、最大步数、工作区、提交、摘要和验证证据。

## 推荐流程

```powershell
vcode task create "重构用户认证并补齐测试"
vcode task plan <task-id>
vcode task approve <task-id>
vcode task run <task-id>
vcode task show <task-id> --json
vcode task merge <task-id>
```

规划阶段会生成两个并行只读角色：代码结构探索和测试/验证入口检查。规划角色必须综合两份报告，用中文写清目标、范围、步骤、原因和验证方式。批准前，任务停在阻断状态，不能执行写入节点。

Build 节点使用独立 Git worktree；完成后记录提交，`task merge` 才会把提交集成回项目。冲突会自动中止 cherry-pick 并把任务标记为阻断。重复执行 merge 不会重复应用已经集成的节点。

## 完成状态

- `VERIFIED`：相关项目检查全部成功。
- `PARTIAL`：代码或部分检查完成，但存在失败/未完成证据。
- `UNVERIFIED`：没有可靠检查，或使用了 `--no-verify`。

只有 `VERIFIED` 才会输出“completed”；其他情况会明确显示下一步。`doctor --json` 可用于 CI 读取配置来源、模型、工具、实际沙箱状态和降级原因。

## 角色工具范围

可以在 `vcode.toml` 中通过 `[agent.roles.<role>]` 配置模型、最大步数和工具白名单。规划、探索、审查角色即使误配了写工具，运行时仍会经过只读计划模式拦截。
