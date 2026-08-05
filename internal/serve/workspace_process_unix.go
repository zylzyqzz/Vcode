//go:build !windows

package serve

import "syscall"

func workspaceProcessAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
