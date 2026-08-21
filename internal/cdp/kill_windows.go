//go:build windows

package cdp

import (
	"os/exec"
	"strconv"
	"syscall"
)

// killTree uses taskkill /T /F to terminate the process and its children.
func killTree(cmd *exec.Cmd) error {
	return exec.Command("taskkill", "/T", "/F", "/PID", itoa(cmd.Process.Pid)).Run()
}

// setChildProcessGroup is a no-op on Windows; taskkill handles the tree.
func setChildProcessGroup(cmd *exec.Cmd) {}

func itoa(n int) string { return strconv.Itoa(n) }

// isProcessAlive checks process existence via the Windows OpenProcess API.
func isProcessAlive(cmd *exec.Cmd) bool {
	if cmd.Process == nil {
		return false
	}
	if cmd.ProcessState != nil {
		return false
	}
	// OpenProcess with PROCESS_QUERY_LIMITED_INFORMATION (0x1000); a live
	// process returns a valid handle (which we immediately close).
	const processQueryLimitedInformation = 0x1000
	const errInvalidParameter syscall.Errno = 87
	h, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(cmd.Process.Pid))
	if err != nil {
		// ERROR_INVALID_PARAMETER (87) means the pid doesn't exist.
		if err == errInvalidParameter {
			return false
		}
		return false
	}
	_ = syscall.CloseHandle(h)
	return true
}
