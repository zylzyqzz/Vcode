# 005 — 长任务 CLI 续航审计

## 本轮范围

本轮继续围绕 Vcode 的 CLI-only 目标，验证长任务、多智能体、Windows 和发布链路。桌面端配置读取仍保留，但桌面发布 workflow 已停止触发，避免影响 CLI 发布。

## 已验证

- `internal/taskgraph`：依赖图、并行执行上限、重试、取消恢复、部分成功、生命周期事件、摘要/令牌遥测和恢复后持久化。
- `internal/verify`：Go、Node、Python、Rust 项目命令识别；无命令返回 `UNVERIFIED`；命令失败、取消和超时保留明确证据。
- `internal/cli`、`internal/sandbox`、`internal/permission`：紧凑底栏、工具/思考折叠、中文规划、Windows 降级沙箱、危险命令拦截和 ANSI/路径回归。
- 版本注入和交叉构建：Windows/Linux/macOS 的 amd64 与 arm64 均可生成 CLI 二进制。
- OpenAI/DeepSeek 流式取消：Windows 下取消不会等待重连或永久阻塞。

## 当前产品边界

- 主产品是 Go 终端 CLI；桌面端不参与默认构建、测试和发布。
- 短任务默认保持轻量；长任务通过 `vcode task plan/approve/run/merge` 使用持久化任务图和隔离 worktree。
- 任务结果必须落在 `VERIFIED`、`PARTIAL` 或 `UNVERIFIED`，没有证据时不报告成功。
- `--no-verify` 是显式跳过，并会留下 `UNVERIFIED` 记录。

## 后续验收

最终轮仍需在本机执行全量 `go test ./...`、`go vet ./...`、CLI 安装 smoke test，并确认所有本轮提交已推送到远端分支。
