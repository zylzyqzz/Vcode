# 006 — CLI 长任务交付验收

## 结果

- 全量 `go test ./... -count=1 -timeout=8m`：通过。
- 全量 `go vet ./...`：通过。
- Windows amd64/arm64、Linux amd64/arm64、macOS amd64/arm64：交叉构建通过。
- 本机 CLI：已覆盖安装到 `C:\Program Files\Vcode\vcode.exe`。
- 本机 `vcode version`：`0511b3e`。
- 本机 `vcode doctor --json`：DeepSeek provider、`deepseek-v4-flash`、配置来源和权限状态均可见。
- 远端分支：`agent/vcode-cli-ui-polish` 已推送并与本地一致。

## Windows 安全边界

本机没有 OS 级 Bash 沙箱，实际状态为 `bash=auto`、`effective=permission-gated`、`degraded=true`、`available=false`。写入根目录、权限确认和危险命令拦截继续生效；用户能看到这是降级保护，而不是完整隔离。

## 长任务能力

持久化任务图、中文只读规划、批准后执行、角色化工具面、独立 worktree、并行节点、重试、取消恢复、部分成功、验证证据和显式 merge 已纳入 CLI 主路径。没有验证证据时，任务不会报告 `VERIFIED`。
