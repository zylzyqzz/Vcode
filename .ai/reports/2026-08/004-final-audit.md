# 004 — 第 21–30 轮交付审计

## 已验证

- `internal/cli` 全量测试通过，包含紧凑底栏、隐藏 Todo 面板、Ctrl+O 折叠和主题契约。
- `internal/taskgraph`、`worktree`、`verify`、`boot`、`config`、`sandbox`、`doctor` 全部通过。
- Windows amd64/arm64、Linux amd64/arm64、macOS amd64/arm64 均成功交叉构建。
- 本机 `C:\Program Files\Vcode\vcode.exe` 已更新；`version` 和 `doctor --json` smoke test 通过。
- 当前 Windows 环境的实际安全状态为 `auto → permission-gated`、`degraded=true`、`available=false`，没有伪装成 OS 隔离。

## 未通过或超时

- `internal/control` 中两个旧测试仍要求历史的“工具自动批准仍触发旧计划审批”语义；当前 CLI 采用显式 `task approve` 和紧凑计划流程，需后续决定是否删除旧测试或兼容该交互。
- `internal/provider/openai/TestStreamCancelDoesNotReconnect` 在 Windows 全量运行中超过 90 秒，测试服务器连接未及时释放；这是跨平台测试基础设施问题，需单独修复，不能宣称全量 `go test ./...` 通过。

## 交付状态

本轮代码、文档、示例、构建产物和本机安装均已完成；上述两项遗留回归已显式记录在项目审计中。
