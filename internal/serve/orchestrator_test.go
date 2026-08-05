package serve

import "testing"

func TestShouldUsePipeline(t *testing.T) {
	for _, input := range []string{"重构认证模块并补齐测试", "migrate the storage layer", "请同时修改前后端并运行全流程验证"} {
		if !shouldUsePipeline(input) {
			t.Errorf("expected pipeline for %q", input)
		}
	}
	for _, input := range []string{"你好", "读取 README.md", "运行 go test ./..."} {
		if shouldUsePipeline(input) {
			t.Errorf("unexpected pipeline for %q", input)
		}
	}
}
