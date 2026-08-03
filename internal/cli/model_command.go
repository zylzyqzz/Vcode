package cli

import (
	"fmt"
	"os"
	"strings"

	"vcode/internal/config"
)

// modelCommand manages the default model without opening an interactive chat.
// Session-local switching remains available through /model.
func modelCommand(args []string) int {
	if len(args) == 0 || strings.EqualFold(args[0], "list") {
		return listConfiguredModels()
	}
	if strings.EqualFold(args[0], "use") && len(args) == 2 {
		return useConfiguredModel(args[1])
	}
	fmt.Fprintln(os.Stderr, "用法：vcode model list | vcode model use <模型>")
	return 2
}

func listConfiguredModels() int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "配置加载失败：", err)
		return 1
	}
	current := cfg.DefaultModel
	fmt.Println("可用模型：")
	seen := map[string]bool{}
	for _, p := range cfg.Providers {
		for _, model := range p.ChatModelList() {
			ref := p.Name + "/" + model
			if seen[ref] {
				continue
			}
			seen[ref] = true
			marker := "  "
			if ref == current || model == current || p.Name == current {
				marker = "✓ "
			}
			fmt.Printf("%s%s\n", marker, brandAccent(ref))
		}
	}
	fmt.Printf("\n当前默认：%s\n", brandAccent(current))
	return 0
}

func useConfiguredModel(ref string) int {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		fmt.Fprintln(os.Stderr, "模型不能为空")
		return 2
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "配置加载失败：", err)
		return 1
	}
	if _, ok := cfg.ResolveModel(ref); !ok {
		fmt.Fprintf(os.Stderr, "未找到模型 %q，请先运行 `vcode model list`。\n", ref)
		return 2
	}
	path := config.UserConfigPath()
	if path == "" {
		fmt.Fprintln(os.Stderr, "无法定位用户配置文件")
		return 1
	}
	edit := config.LoadForEdit(path)
	if err := edit.SetDefaultModel(ref); err != nil {
		fmt.Fprintln(os.Stderr, "模型设置失败：", err)
		return 1
	}
	if err := edit.SaveTo(path); err != nil {
		fmt.Fprintln(os.Stderr, "配置保存失败：", err)
		return 1
	}
	fmt.Printf("默认模型已切换为 %s\n", brandAccent(ref))
	return 0
}
