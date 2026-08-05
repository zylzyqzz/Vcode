//go:build windows

package serve

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func workspaceProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %s", strconv.Itoa(pid)), "/FO", "CSV", "/NH").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), fmt.Sprintf(`"%d"`, pid))
}
