package sandbox

import "testing"

func TestCheckFallbackCommandBlocksDestructiveCommands(t *testing.T) {
	for _, command := range []string{"rm -rf /", "Remove-Item -Recurse C:\\", "Remove-Item -Recurse -Force C:\\", "del /f /s /q C:\\", "shutdown /s /t 0", "format C:"} {
		if err := CheckFallbackCommand(command); err == nil {
			t.Fatalf("command %q was not blocked", command)
		}
	}
}

func TestCheckFallbackCommandAllowsNormalDevelopmentCommands(t *testing.T) {
	for _, command := range []string{"go test ./...", "npm run build", "git status", "Remove-Item .\\tmp.txt"} {
		if err := CheckFallbackCommand(command); err != nil {
			t.Fatalf("command %q was blocked: %v", command, err)
		}
	}
}

func TestStatusKeepsExplicitModesDistinct(t *testing.T) {
	if got := Status("off"); got.Effective != "unconfined" || !got.Degraded {
		t.Fatalf("off status = %+v", got)
	}
	if got := Status("enforce"); got.Effective == "permission-gated" {
		t.Fatalf("enforce unexpectedly degraded to permission-gated: %+v", got)
	}
}
