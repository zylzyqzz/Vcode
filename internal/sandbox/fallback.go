package sandbox

import (
	"fmt"
	"strings"
)

// CheckFallbackCommand blocks a small set of unambiguously destructive host
// commands when auto mode has no OS jail. Normal writes still flow through the
// existing permission policy; this guard is intentionally narrow and is not
// presented as a replacement for OS isolation.
func CheckFallbackCommand(command string) error {
	lower := strings.ToLower(strings.TrimSpace(command))
	patterns := []string{
		"format ", "diskpart", "shutdown ", "shutdown.exe", "reboot ",
		"reg delete", "rm -rf /", "rm -rf \\", "rm -rf c:",
		"del /s /q c:\\", "rd /s /q c:\\", "rmdir /s /q c:\\",
		"remove-item -recurse c:\\", "remove-item -recurse /",
		"mkfs.", "dd if=", "> /dev/sd",
	}
	for _, pattern := range patterns {
		if strings.Contains(lower, pattern) {
			return fmt.Errorf("blocked potentially destructive command in degraded sandbox mode: %s", pattern)
		}
	}
	return nil
}
