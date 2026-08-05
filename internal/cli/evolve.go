package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"vcode/internal/evolution"
)

func evolveCommand(args []string) int {
	if len(args) == 0 {
		evolveUsage()
		return 2
	}
	switch args[0] {
	case "init":
		return evolveInit(args[1:])
	case "benchmark":
		return evolveBenchmark(args[1:])
	case "baseline":
		return evolveBaseline(args[1:])
	case "run":
		return evolveRun(args[1:])
	case "compare":
		return evolveCompare(args[1:])
	case "history":
		return evolveHistory(args[1:])
	case "rollback":
		return evolveRollback(args[1:])
	case "status":
		return evolveStatus(args[1:])
	case "help", "--help", "-h":
		evolveUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown evolve subcommand %q\n\n", args[0])
		evolveUsage()
		return 2
	}
}

func evolveInit(args []string) int {
	fs := flag.NewFlagSet("evolve init", flag.ContinueOnError)
	dir := fs.String("dir", ".", "项目根目录")
	agent := fs.String("agent", evolution.DefaultAgent, "要进化的 Agent 角色")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	root, store, rc := loadEvolutionStore(*dir)
	if rc != 0 {
		return rc
	}
	if err := store.Init(*agent); err != nil {
		fmt.Fprintf(os.Stderr, "error: initialize evolution state: %v\n", err)
		return 1
	}
	state, _ := store.LoadState(*agent)
	fmt.Printf("已初始化 Vcode 自进化状态\n项目: %s\nAgent: %s\n版本: v%d\n目录: %s\n", root, state.Agent, state.Version, store.Root)
	fmt.Println("下一步：vcode evolve benchmark add PATH，然后运行 vcode evolve baseline")
	return 0
}

func evolveBenchmark(args []string) int {
	if len(args) == 0 {
		evolveBenchmarkUsage()
		return 2
	}
	switch args[0] {
	case "list":
		return evolveBenchmarkList(args[1:])
	case "add":
		return evolveBenchmarkAdd(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown benchmark subcommand %q\n", args[0])
		evolveBenchmarkUsage()
		return 2
	}
}

func evolveBenchmarkList(args []string) int {
	fs := flag.NewFlagSet("evolve benchmark list", flag.ContinueOnError)
	dir := fs.String("dir", ".", "项目根目录")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	_, store, rc := loadEvolutionStore(*dir)
	if rc != 0 {
		return rc
	}
	benchmarks, err := store.ListBenchmarks()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: list benchmarks: %v\n", err)
		return 1
	}
	if len(benchmarks) == 0 {
		fmt.Println("暂无 Benchmark")
		return 0
	}
	for _, benchmark := range benchmarks {
		fmt.Printf("%s\tv%d\t%d cases\n", benchmark.Name, benchmark.Version, len(benchmark.Cases))
	}
	return 0
}

func evolveBenchmarkAdd(args []string) int {
	fs := flag.NewFlagSet("evolve benchmark add", flag.ContinueOnError)
	dir := fs.String("dir", ".", "项目根目录")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: vcode evolve benchmark add [--dir PATH] BENCHMARK.toml")
		return 2
	}
	_, store, rc := loadEvolutionStore(*dir)
	if rc != 0 {
		return rc
	}
	benchmark, err := store.AddBenchmark(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: add benchmark: %v\n", err)
		return 1
	}
	fmt.Printf("已添加 Benchmark：%s（%d cases）\n", benchmark.Name, len(benchmark.Cases))
	return 0
}

func evolveStatus(args []string) int {
	fs := flag.NewFlagSet("evolve status", flag.ContinueOnError)
	dir := fs.String("dir", ".", "项目根目录")
	agent := fs.String("agent", evolution.DefaultAgent, "Agent 角色")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	_, store, rc := loadEvolutionStore(*dir)
	if rc != 0 {
		return rc
	}
	state, err := store.LoadState(*agent)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Println("自进化未初始化。运行：vcode evolve init")
			return 1
		}
		fmt.Fprintf(os.Stderr, "error: load evolution status: %v\n", err)
		return 1
	}
	history, err := store.ListHistory()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: load evolution history: %v\n", err)
		return 1
	}
	fmt.Printf("Agent: %s\nVersion: v%d\nStatus: %s\nHistory: %d runs\n", state.Agent, state.Version, state.Status, len(history))
	if state.AcceptedRun != "" {
		fmt.Printf("Accepted run: %s\n", state.AcceptedRun)
	}
	return 0
}

func evolveHistory(args []string) int {
	fs := flag.NewFlagSet("evolve history", flag.ContinueOnError)
	dir := fs.String("dir", ".", "项目根目录")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	_, store, rc := loadEvolutionStore(*dir)
	if rc != 0 {
		return rc
	}
	history, err := store.ListHistory()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: load evolution history: %v\n", err)
		return 1
	}
	for _, run := range history {
		fmt.Printf("%s\tv%d\t%s\t%.2f\t%s\n", run.ID, run.Version, run.Status, run.Score, run.Reason)
	}
	return 0
}

func evolveRollback(args []string) int {
	fs := flag.NewFlagSet("evolve rollback", flag.ContinueOnError)
	dir := fs.String("dir", ".", "项目根目录")
	agent := fs.String("agent", evolution.DefaultAgent, "Agent 角色")
	if err := fs.Parse(args); err != nil || fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: vcode evolve rollback [--dir PATH] [--agent build] VERSION")
		return 2
	}
	_, store, rc := loadEvolutionStore(*dir)
	if rc != 0 {
		return rc
	}
	state, err := store.LoadState(*agent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: load evolution state: %v\n", err)
		return 1
	}
	if _, err := store.Snapshot(*agent); err != nil {
		fmt.Fprintf(os.Stderr, "error: snapshot current version: %v\n", err)
		return 1
	}
	version, err := strconv.Atoi(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid version %q\n", fs.Arg(0))
		return 2
	}
	if err := store.Rollback(*agent, version); err != nil {
		fmt.Fprintf(os.Stderr, "error: rollback v%d: %v\n", version, err)
		return 1
	}
	state.Version = version
	state.Status = "rolled_back"
	if err := store.SaveState(state); err != nil {
		fmt.Fprintf(os.Stderr, "error: save rollback state: %v\n", err)
		return 1
	}
	fmt.Printf("已回滚 %s Agent 到 v%d\n", state.Agent, version)
	return 0
}

func evolveBaseline(args []string) int {
	fs := flag.NewFlagSet("evolve baseline", flag.ContinueOnError)
	dir := fs.String("dir", ".", "项目根目录")
	agent := fs.String("agent", evolution.DefaultAgent, "Agent 角色")
	benchmark := fs.String("benchmark", "", "Benchmark 名称")
	repeats := fs.Int("repeats", evolution.DefaultRepeats, "每个任务重复评测次数")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	_, store, rc := loadEvolutionStore(*dir)
	if rc != 0 {
		return rc
	}
	if strings.TrimSpace(*benchmark) == "" {
		fmt.Fprintln(os.Stderr, "error: --benchmark is required")
		return 2
	}
	if *repeats < 1 {
		fmt.Fprintln(os.Stderr, "error: --repeats must be positive")
		return 2
	}
	ctx := context.Background()
	root, _ := filepath.Abs(*dir)
	run, err := store.RunBaseline(ctx, root, *agent, *benchmark, *repeats)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: baseline failed: %v\n", err)
		return 1
	}
	fmt.Printf("Baseline 完成：%s\n版本：v%d\n平均分：%.2f\n任务数：%d\n", run.ID, run.Version, run.Score, len(run.Cases))
	return 0
}

func evolveRun(args []string) int {
	fs := flag.NewFlagSet("evolve run", flag.ContinueOnError)
	dir := fs.String("dir", ".", "项目根目录")
	benchmark := fs.String("benchmark", "", "Benchmark 名称")
	rounds := fs.Int("rounds", evolution.DefaultRounds, "最大进化轮数")
	repeats := fs.Int("repeats", evolution.DefaultRepeats, "每个任务重复评测次数")
	agent := fs.String("agent", evolution.DefaultAgent, "Agent 角色")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*benchmark) == "" || *rounds < 1 || *repeats < 1 {
		fmt.Fprintln(os.Stderr, "error: --benchmark is required; rounds and repeats must be positive")
		return 2
	}
	root, store, rc := loadEvolutionStore(*dir)
	if rc != 0 {
		return rc
	}
	if _, err := store.LoadState(*agent); err != nil {
		fmt.Fprintln(os.Stderr, "error: evolution is not initialized; run `vcode evolve init` first")
		return 1
	}
	runs, err := store.RunEvolution(context.Background(), root, *agent, *benchmark, *rounds, *repeats)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: evolution failed: %v\n", err)
		return 1
	}
	for _, run := range runs {
		fmt.Printf("%s\tversion v%d\tscore %.2f\t%s\n", run.ID, run.Version, run.Score, run.Reason)
	}
	return 0
}

func evolveCompare(args []string) int {
	fs := flag.NewFlagSet("evolve compare", flag.ContinueOnError)
	dir := fs.String("dir", ".", "项目根目录")
	if err := fs.Parse(args); err != nil || fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: vcode evolve compare [--dir PATH] RUN_ID")
		return 2
	}
	_, store, rc := loadEvolutionStore(*dir)
	if rc != 0 {
		return rc
	}
	run, err := store.LoadRun(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: load run: %v\n", err)
		return 1
	}
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: encode run: %v\n", err)
		return 1
	}
	fmt.Println(string(data))
	return 0
}

func evolveExecutionPlaceholder(command string, args []string) int {
	// Execution is deliberately kept behind the phase-two runner until isolated
	// workspaces and rubric redaction are available. This prevents a partially
	// wired command from mutating a real project without evidence or rollback.
	fs := flag.NewFlagSet("evolve "+command, flag.ContinueOnError)
	dir := fs.String("dir", ".", "项目根目录")
	if command == "run" {
		fs.Int("rounds", evolution.DefaultRounds, "最大进化轮数")
		fs.Int("repeats", evolution.DefaultRepeats, "每个任务重复评测次数")
	}
	_ = fs.String("benchmark", "", "Benchmark 名称")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	_, store, rc := loadEvolutionStore(*dir)
	if rc != 0 {
		return rc
	}
	if _, err := os.Stat(store.Root); err != nil {
		fmt.Fprintln(os.Stderr, "error: evolution is not initialized; run `vcode evolve init` first")
		return 1
	}
	fmt.Fprintf(os.Stderr, "error: evolve %s runner is not enabled yet; state and rollback are ready, benchmark execution is the next stage\n", command)
	return 1
}

func loadEvolutionStore(dir string) (string, *evolution.Store, int) {
	root, err := filepath.Abs(strings.TrimSpace(dir))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: resolve project directory: %v\n", err)
		return "", nil, 1
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "error: project directory does not exist: %s\n", root)
		return "", nil, 1
	}
	return root, evolution.NewStore(root), 0
}

func evolveUsage() {
	fmt.Print(`vcode evolve — controlled Build Agent self-improvement

Usage:
  vcode evolve init [--dir PATH] [--agent build]
  vcode evolve benchmark list|add PATH
  vcode evolve baseline [--dir PATH] [--benchmark NAME]
  vcode evolve run [--dir PATH] [--benchmark NAME] [--rounds 3] [--repeats 2]
  vcode evolve compare [--dir PATH] [RUN_ID]
  vcode evolve history [--dir PATH]
  vcode evolve rollback [--dir PATH] [--agent build] VERSION
  vcode evolve status [--dir PATH] [--agent build]
`)
}

func evolveBenchmarkUsage() {
	fmt.Print(`Usage:
  vcode evolve benchmark list [--dir PATH]
  vcode evolve benchmark add [--dir PATH] BENCHMARK.toml
`)
}
