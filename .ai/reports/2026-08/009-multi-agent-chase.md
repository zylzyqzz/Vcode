# 009 — 多智能体追赶轮次验收

## 本轮交付

- Coordinator 决策契约：动态加节点、重试、等待、取消，并在落盘前完整校验。
- Blackboard：事实、来源、节点和置信度持久化，生成结构化观察事件。
- Agent mailbox：点对点/广播消息、投递确认和恢复后继续消费。
- Agent presence：心跳、角色、节点、状态和错误。
- 失败恢复：超时、网络、认证、权限、编译、测试、冲突、取消和预算分类。
- worktree：冲突文件报告和 cherry-pick 自动回滚。
- checkpoint：保存/恢复节点状态、Outcome 和 Blackboard。
- CLI：`task events` 和 `task agents` 的文本/JSON 查询。

## 验证

- `go test ./... -count=1 -timeout=8m`：通过。
- `go vet ./...`：通过。
- Windows amd64/arm64、Linux amd64/arm64、macOS amd64/arm64：通过。
- `git diff --check`：通过。

## 当前边界

Coordinator 现在已经是可持久化的控制面，但模型驱动的动态拆解仍需在后续真实项目长跑中继续校准；未知错误不会被静默标记为成功，权限、取消和预算耗尽会进入等待/人工处理路径。
