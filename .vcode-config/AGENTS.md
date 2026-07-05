# Vcode — DeepSeek-native AI coding agent

## 项目概述

Vcode 是一个基于 Go 的终端 AI 编程代理，针对 DeepSeek 模型优化。核心特点是利用 DeepSeek 的 prefix-cache 来降低长会话成本。

## 目录结构

```
cmd/Vcode/               # 主 CLI 入口
cmd/Vcode-plugin-example/ # 插件示例
internal/                   # 核心逻辑
  agent/                    # 代理运行时（会话、工具执行、消息处理）
  boot/                     # 启动引导
  cli/                      # CLI 命令路由
  config/                   # 配置解析 (TOML)
  provider/                 # 模型供应商适配
  tool/                     # 内置工具
  plugin/                   # 插件系统 (MCP)
  skill/                    # 技能系统
  memory/                   # 上下文管理
  planmode/                 # Plan 模式
  sandbox/                  # 沙箱安全
  permission/               # 权限控制
  checkpoint/               # 编辑回滚/撤销
  lsp/                      # 语言服务器
  jobs/                     # 后台任务
docs/                       # 文档
desktop/                    # 桌面应用 (Electron)
npm/                        # npm 包装器
```

## 构建与运行

```bash
# 构建二进制
go build -o bin/Vcode.exe ./cmd/Vcode

# 运行
Vcode                    # 交互式会话
Vcode run "任务"         # 单次执行
Vcode setup              # 配置向导
```

## 配置

配置文件位置: `%AppData%/Vcode/config.toml` (Windows)
               `~/.Vcode/config.toml` (macOS/Linux)

API Key 存在 `%AppData%/Vcode/.env` 中，格式 `DEEPSEEK_API_KEY=sk-xxx`

## 架构要点

- 单 Go 二进制，CGO_ENABLED=0
- 配置驱动（TOML），无硬编码模型
- 插件通过 MCP stdio JSON-RPC 子进程通信
- 内置工具在编译时自注册
- 上下文管理采用 prefix-cache 感知策略