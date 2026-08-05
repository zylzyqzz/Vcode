# Vcode 可控自进化

Vcode 的自进化默认关闭，只有显式运行 `vcode evolve init` 后才会创建状态目录。第一版只允许优化 Build Agent 的 `AGENTS.md`、Build 专属 Skill 和角色版本文件，不会修改 Vcode 核心代码、权限、沙箱或密钥。

## 快速开始

```powershell
vcode evolve init
vcode evolve benchmark list
vcode evolve baseline --benchmark example
vcode evolve run --benchmark example --rounds 3 --repeats 2
vcode evolve status
vcode evolve history
```

`init` 会在 `.vcode/evolution/` 创建可编辑的 Agent 状态、快照目录和一个示例基准。真实项目建议复制示例 Benchmark，填写真实任务、允许修改范围、期望文件和验证命令，再用 `vcode evolve benchmark add PATH` 导入。

## 安全边界

- 每轮进化前都会保存当前 Build Agent 快照。
- 候选只能在隔离目录中修改 `AGENTS.md` 和 `skills/`。
- Benchmark 的 `rubric.md`、其他 case、API Key 和 `.env` 不会进入 Build Agent 上下文或快照。
- 候选必须产生实际文件变化、通过验证、没有越界修改，并且综合分严格高于当前版本才会接受。
- 失败候选自动保留在运行记录中，当前版本保持不变；可用 `vcode evolve rollback VERSION` 恢复已保存版本。

## Benchmark 格式

```toml
name = "my-project"
version = 1

[[cases]]
id = "fix-build"
task = "修复项目的编译错误，并运行验证命令。"
fixture = "benchmarks/fixtures/fix-build"
allowed_paths = ["internal", "cmd"]
expected_files = ["internal/example.go"]
verify_commands = ["go test ./..."]
repeats = 2
```

路径必须是相对路径，不能越过项目根目录。私有评分规则可以放在 Benchmark 外部的评测流程中；Build Agent 只看到任务和 fixture，不看到评分细则。

## 状态与回滚

```text
.vcode/evolution/
├── agents/build/       # 当前 Build overlay 与版本
├── benchmarks/         # 任务基准
├── runs/               # 隔离工作区和评测记录
└── history.jsonl       # 进化历史
```

普通 `vcode` 会话不会读取进化 overlay。只有评测和优化子进程通过内部标记显式启用它。
