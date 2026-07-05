---
name: Vcode-architecture
description: Vcode project architecture overview for the agent
---

## Vcode 架构速查

### 核心流程

```
CLI入口 (cmd/Vcode/main.go)
  → boot.Boot() 初始化配置/供应商/工具
  → cli 路由到对应命令
  → agent.NewSession() 创建代理会话
  → agent.Run() 进入消息循环
    → provider.Chat() 调用 LLM
    → tool.Execute() 执行工具调用
    → memory.Compact() 上下文压缩
```

### 关键包速查

| 包 | 职责 |
|---|---|
| `internal/agent` | 代理运行时：消息循环、会话状态、工具执行编排 |
| `internal/provider` | 模型供应商适配器（OpenAI-compatible） |
| `internal/tool` | 内置工具注册与执行 |
| `internal/plugin` | MCP 插件系统（子进程 stdio JSON-RPC） |
| `internal/skill` | 技能系统（slash commands） |
| `internal/config` | TOML 配置解析 |
| `internal/memory` | 上下文管理与压缩（prefix-cache 感知） |
| `internal/planmode` | Plan 模式（只读规划阶段） |
| `internal/sandbox` | 沙箱安全（macOS Seccomp / Linux Landlock） |
| `internal/permission` | 工具调用权限门控（ask/allow/deny） |
| `internal/checkpoint` | 编辑快照与回滚（Esc+Esc / /rewind） |
| `internal/boot` | 启动引导：加载配置、注册工具、初始化供应商 |

### 数据流

1. 用户输入 → `agent.Session.Run()`
2. 消息加入历史 → `provider.Chat()` 获取 LLM 回复
3. LLM 返回工具调用 → `tool.Execute()` 执行
4. 工具结果加回消息 → 继续循环直到 LLM 返回最终回复
5. 上下文超限 → `memory.Compact()` 压缩旧消息

### 配置加载顺序

flag > ./Vcode.toml > ~/.Vcode/config.toml > 内置默认值