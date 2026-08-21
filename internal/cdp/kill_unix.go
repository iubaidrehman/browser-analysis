//go:build !windows

package cdp

import (
	"os/exec"
	"syscall"
)

// killTree signals the whole process group; the child was started with
// Setpgid so it leads its own group.
func killTree(cmd *exec.Cmd) error {
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
}

// setChildProcessGroup makes the child lead its own process group.
func setChildProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// isProcessAlive on POSIX uses signal 0, which performs error checking only.
func isProcessAlive(cmd *exec.Cmd) bool {
	if cmd.Process == nil {
		return false
	}
	if cmd.ProcessState != nil {
		return false
	}
	return cmd.Process.Signal(syscall.Signal(0)) == nil
}
