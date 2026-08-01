# 003 — 长任务隔离与角色运行时

## 已完成

- 构建节点使用 `.vcode/worktrees/<task>/<node>` 独立 Git 工作区。
- 成功节点记录变更文件和提交，使用 `vcode task merge` 显式集成。
- Cherry-pick 冲突自动中止并把任务标记为阻断，避免污染主工作区。
- Scheduler 支持并发上限、依赖、重试、取消恢复、幂等重跑和生命周期事件。
- `agent.roles.<role>.tools` 可以缩小角色可见工具范围；规划/探索/审查仍保持只读。

## 验证

- taskgraph、worktree、CLI、boot 定向测试通过。
- 覆盖工作区创建/提交/合并/冲突、32 节点长链、并发限制、取消恢复和幂等重跑。

## 后续

- 将任务阶段摘要继续收敛到交互 TUI 的紧凑状态区域。
- 补齐 MCP/Skills 按角色的显式策略和更完整的跨平台 smoke test。
