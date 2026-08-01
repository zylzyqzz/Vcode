<p align="center">
  <img src="docs/logo.svg" alt="Vcode" width="640"/>
</p>

<p align="center">
  <a href="./README.md">English</a>
  &nbsp;·&nbsp;
  <strong>简体中文</strong>
  &nbsp;·&nbsp;
  <a href="./docs/GUIDE.zh-CN.md">指南</a>
  &nbsp;·&nbsp;
  <a href="./docs/SPEC.md">规格</a>
</p>

<p align="center">面向终端的 DeepSeek 原生 AI coding agent。</p>
<p align="center">由配置与插件驱动的极薄 harness——单一静态 Go 二进制，围绕 DeepSeek 的前缀缓存调优，长会话也能把 token 成本压低。</p>

<br/>

## 特性

- **配置驱动**：provider、agent、启用的工具、插件全部在 `vcode.toml` 中声明，内核无硬编码模型。
- **多模型 · 可组合**：DeepSeek 作为预设内置；任何 OpenAI 兼容端点都只是一条配置。可选让两个模型协同（执行器 + 规划器），各自独立、缓存稳定的 session。
- **插件驱动**：外部工具以子进程形式运行，通过 stdio JSON-RPC 通信（MCP 兼容）；内置工具在编译期自注册。
- **缓存友好的上下文维护**：启动时注入稳定的环境摘要；旧工具输出会先 snip/prune，再进入摘要 compaction；内置工具 schema 合约有文档和回归测试保护。
- **零摩擦分发**：`CGO_ENABLED=0` 单二进制；一条命令交叉编译到六个目标平台。唯一依赖是一个 TOML 解析库。

## 安装

```sh
npm i -g Vcode                  # 任意系统;自动拉取对应平台的原生二进制
brew install esengine/Vcode/Vcode   # macOS
```

预编译归档(`darwin|linux|windows × amd64|arm64`)和 `SHA256SUMS` 见每个 [GitHub release](https://github.com/zylzyqzz/Vcode-go/releases)。

### 代码签名

Windows 构建使用 [SignPath 基金会](https://signpath.org/) 提供的免费代码签名证书，通过 [SignPath.io](https://signpath.io/) 完成签名。

### 从源码构建

```sh
make build      # -> bin/Vcode(.exe)
make cross      # -> dist/（darwin|linux|windows × amd64|arm64）
```

## 快速开始

```sh
首次运行 Vcode                     # 只输入 DeepSeek API Key，自动配置 flash/pro
Vcode setup                      # 随时重新运行同一套最简配置引导
export DEEPSEEK_API_KEY=sk-...      # 也可以让 setup 保存到 Vcode 全局 .env
Vcode                            # 然后在会话里运行 /init 生成 AGENTS.md（项目记忆）
Vcode run "把 main.go 里的 TODO 实现掉"
Vcode run --model deepseek-pro "给这个函数补单元测试"
echo "解释这段代码" | Vcode run
```

`vcode run` 会自动识别常见项目检查，并在任务结束时报告 `VERIFIED`、`PARTIAL`
或 `UNVERIFIED`；如需明确跳过可使用 `--no-verify`。Bash 默认使用
`sandbox.bash = "auto"`：Windows 没有 OS 级沙箱时仍可使用，但 Vcode 会明确提示
此时由权限策略承担主要安全边界。

## 配置

一个最小的 `vcode.toml`——一个 provider 加一个默认模型——就够跑起来:

```toml
default_model = "deepseek-flash"

[[providers]]
name        = "deepseek-flash"
kind        = "openai"
base_url    = "https://api.deepseek.com"
model       = "deepseek-v4-flash"
api_key_env = "DEEPSEEK_API_KEY"
```

优先级为 **flag > `./vcode.toml` > 用户配置文件 > 内置默认值**；从 **Vcode v1.8.1** 开始，用户配置位于 macOS/Linux 的 `~/.Vcode/config.toml`，Windows 为 `%AppData%\Vcode\config.toml`。迁移细节见 **[配置路径](./docs/CONFIG_PATHS.zh-CN.md)**，其中也说明了全局 `config.toml` 和 `.env` 的完整结构。权限、沙盒、插件(MCP)、斜杠命令、`@` 引用与双模型设置，全部在 **[指南](./docs/GUIDE.zh-CN.md)** 里。

## 文档

- **[指南](./docs/GUIDE.zh-CN.md)** —— 配置、权限与沙盒、插件(MCP)、斜杠命令、`@` 引用、双模型协同。
- **[机器人使用指南](./docs/BOT_GUIDE.zh-CN.md)** —— 通过终端 Bot 网关连接飞书、Lark、微信 Bot，以及 IM 里的审批、YOLO 和命令交互。
- **[规格](./docs/SPEC.md)** —— 工程契约:架构、registry、数据类型与路线图。
- **[工具合约](./docs/TOOL_CONTRACT.zh-CN.md)** —— provider 可见的内置工具名、read-only 标记和 schema 快照保护。
- **[Checkpoints 与 rewind](./docs/CHECKPOINTS.md)** —— 基于快照的编辑安全网 (Esc-Esc / `/rewind`)。

<br/>

## 许可证

MIT —— 见 [LICENSE](./LICENSE)
## 长任务与多智能体

Vcode 将长任务保存为 `.vcode/tasks/` 下的持久化任务图。推荐先运行 `vcode task plan <id>` 生成中文只读计划，再运行 `vcode task approve <id>` 批准写入，最后运行 `vcode task run <id>` 和 `vcode task merge <id>`。实现节点在独立 Git worktree 中执行，冲突会阻断而不会覆盖主工作区。

Vcode 当前只维护终端 CLI；桌面客户端、桌面构建和桌面发布链已移除。旧配置字段仍按兼容策略读取，不影响已有用户升级。

详细流程见 [长任务与多智能体指南](./docs/LONG_TASKS.zh-CN.md)。
