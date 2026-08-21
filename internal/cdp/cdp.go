// Package cdp implements scenario D: Chromium is spawned independently and
// controlled through the Chrome DevTools Protocol (CDP).
package cdp

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mxschmitt/playwright-go"
)

// Launcher spawns a raw Chromium process with a CDP debugging port and
// tracks it for cleanup.
type Launcher struct {
	cmd         *exec.Cmd
	port        int
	userDataDir string
}

// Launch starts Chromium with --remote-debugging-port=0 (auto-assigned) and
// returns the Launcher, the resolved debug port, and the spawn-to-ready
// duration once the DevToolsActivePort file appears.
func Launch(executable string, headless bool) (*Launcher, int, time.Duration, error) {
	userDataDir, err := os.MkdirTemp("", "bcrl-cdp-*")
	if err != nil {
		return nil, 0, 0, err
	}

	args := []string{
		"--remote-debugging-port=0",
		"--user-data-dir=" + userDataDir,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-background-networking",
		"about:blank",
	}
	if headless {
		args = append(args, "--headless=new")
	}

	cmd := exec.Command(executable, args...)
	setChildProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(userDataDir)
		return nil, 0, 0, fmt.Errorf("spawn chromium: %w", err)
	}
	l := &Launcher{cmd: cmd, userDataDir: userDataDir}

	// The debug port is written to <user-data-dir>/DevToolsActivePort. The
	// spawn-to-ready interval is the raw browser startup measurement.
	start := time.Now()
	port, err := l.waitForPort(userDataDir)
	dur := time.Since(start)
	if err != nil {
		_ = l.Kill()
		return nil, 0, 0, err
	}
	return l, port, dur, nil
}

// waitForPort polls the DevToolsActivePort file written by Chromium.
func (l *Launcher) waitForPort(userDataDir string) (int, error) {
	portFile := filepath.Join(userDataDir, "DevToolsActivePort")
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		f, err := os.Open(portFile)
		if err == nil {
			sc := bufio.NewScanner(f)
			if sc.Scan() {
				port, perr := strconv.Atoi(strings.TrimSpace(sc.Text()))
				f.Close()
				if perr == nil && port > 0 {
					return port, nil
				}
			} else {
				f.Close()
			}
		}
		// If the process died early, surface the exit error.
		if !l.running() {
			return 0, fmt.Errorf("chromium exited before DevToolsActivePort appeared")
		}
		time.Sleep(100 * time.Millisecond)
	}
	return 0, fmt.Errorf("timed out waiting for DevToolsActivePort")
}

// running reports whether the spawned process is still alive.
func (l *Launcher) running() bool {
	if l.cmd == nil || l.cmd.Process == nil {
		return false
	}
	return isProcessAlive(l.cmd)
}

// Port returns the assigned CDP port.
func (l *Launcher) Port() int { return l.port }

// Kill terminates the Chromium process tree and removes the user-data dir.
func (l *Launcher) Kill() error {
	var killErr error
	if l.cmd != nil && l.cmd.Process != nil {
		if err := killTree(l.cmd); err != nil {
			killErr = err
			_ = l.cmd.Process.Kill()
		}
	}
	if l.userDataDir != "" {
		_ = os.RemoveAll(l.userDataDir)
	}
	return killErr
}

// ConnectOverCDP connects Playwright to the running Chromium via its CDP
// endpoint. The returned close func tears down the connection.
func ConnectOverCDP(pw *playwright.Playwright, port int) (playwright.Browser, func() error, error) {
	wsEndpoint := fmt.Sprintf("http://127.0.0.1:%d", port)
	browser, err := pw.Chromium.ConnectOverCDP(wsEndpoint)
	if err != nil {
		return nil, nil, fmt.Errorf("connect over cdp: %w", err)
	}
	closeFn := func() error { return browser.Close() }
	return browser, closeFn, nil
}
