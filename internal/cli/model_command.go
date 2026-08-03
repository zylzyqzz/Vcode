package cli

import (
	"flag"
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
	if strings.EqualFold(args[0], "add") {
		return addDeepSeekPlatform(args[1:])
	}
	fmt.Fprintln(os.Stderr, "用法：vcode model list | vcode model use <模型> | vcode model add [选项]")
	return 2
}

func addDeepSeekPlatform(args []string) int {
	fs := flag.NewFlagSet("vcode model add", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	name := fs.String("name", "", "平台名称，例如 company-gateway")
	baseURL := fs.String("base-url", "", "OpenAI 兼容 API 地址")
	model := fs.String("model", "deepseek-v4-flash", "DeepSeek 模型：deepseek-v4-flash 或 deepseek-v4-pro")
	keyEnv := fs.String("api-key-env", "", "API Key 环境变量名")
	makeDefault := fs.Bool("default", true, "设为默认模型")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *name == "" || *baseURL == "" {
		fmt.Fprintln(os.Stderr, "必须提供 --name 和 --base-url")
		fmt.Fprintln(os.Stderr, "示例：vcode model add --name my-gateway --base-url https://example.com/v1")
		return 2
	}
	modelID := strings.ToLower(strings.TrimSpace(*model))
	if modelID != "deepseek-v4-flash" && modelID != "deepseek-v4-pro" {
		fmt.Fprintln(os.Stderr, "只支持 DeepSeek 模型：deepseek-v4-flash 或 deepseek-v4-pro")
		return 2
	}
	keyName := strings.TrimSpace(*keyEnv)
	if keyName == "" {
		keyName = "VCODE_" + strings.ToUpper(strings.NewReplacer("-", "_", ".", "_", " ", "_").Replace(*name)) + "_API_KEY"
	}
	if key := strings.TrimSpace(os.Getenv(keyName)); key == "" && isInteractive() {
		fmt.Printf("请输入 %s 的 API Key（明文）：\n", *name)
		key, err := readAPIKey(os.Stdin, os.Stdout)
		if err != nil || strings.TrimSpace(key) == "" {
			fmt.Fprintln(os.Stderr, "未输入 API Key，已取消。")
			return 1
		}
		if _, err := config.StoreCredentialLines([]string{keyName + "=" + key}); err != nil {
			fmt.Fprintln(os.Stderr, "保存 API Key 失败：", err)
			return 1
		}
	}
	path := config.UserConfigPath()
	if path == "" {
		fmt.Fprintln(os.Stderr, "无法定位用户配置文件")
		return 1
	}
	cfg := config.LoadForEdit(path)
	entry := config.ProviderEntry{
		Name: nameValue(*name), Kind: "openai", BaseURL: strings.TrimRight(strings.TrimSpace(*baseURL), "/"),
		Model: modelID, Models: []string{modelID}, Default: modelID, APIKeyEnv: keyName,
		ContextWindow: 1000000,
	}
	if err := cfg.UpsertProvider(entry); err != nil {
		fmt.Fprintln(os.Stderr, "平台配置无效：", err)
		return 1
	}
	if *makeDefault {
		if err := cfg.SetDefaultModel(entry.Name + "/" + modelID); err != nil {
			fmt.Fprintln(os.Stderr, "设置默认模型失败：", err)
			return 1
		}
	}
	if err := cfg.SaveTo(path); err != nil {
		fmt.Fprintln(os.Stderr, "保存配置失败：", err)
		return 1
	}
	fmt.Printf("平台已添加：%s · %s\n", brandAccent(entry.Name), brandAccent(modelID))
	if *makeDefault {
		fmt.Println("已设为默认模型。")
	}
	return 0
}

func nameValue(name string) string {
	return strings.TrimSpace(name)
}

func listConfiguredModels() int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "配置加载失败：", err)
		return 1
	}
	current := cfg.DefaultModel
	fmt.Println("可用模型：")
	for _, ref := range configuredModelRefs(cfg) {
		marker := "  "
		if ref == current {
			marker = "✓ "
		}
		fmt.Printf("%s%s\n", marker, brandAccent(ref))
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
