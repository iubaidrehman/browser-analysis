// Package buildinfo gathers reproducibility metadata: the git commit the
// benchmark was built from, a hash of the effective configuration, and
// detected software versions (spec section 34).
package buildinfo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os/exec"
	"runtime"
	"strings"

	"bcrl/internal/config"
)

// GitCommit returns the current git HEAD commit, suffixed with "-dirty" when
// the working tree has uncommitted changes, or "" if unavailable.
func GitCommit() string {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	commit := strings.TrimSpace(string(out))
	status, err := exec.Command("git", "status", "--porcelain").Output()
	if err == nil && len(strings.TrimSpace(string(status))) > 0 {
		commit += "-dirty"
	}
	return commit
}

// ConfigHash returns a stable SHA-256 prefix of the effective configuration
// plus the experiment mode (mode is not part of the YAML config but changes
// the experiment).
func ConfigHash(cfg config.Config, mode string) string {
	h := sha256.New()
	h.Write([]byte(mode + "\x00"))
	b, err := json.Marshal(cfg)
	if err != nil {
		return ""
	}
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// GoVersion returns the runtime Go version.
func GoVersion() string { return runtime.Version() }

// NodeVersion returns the node --version output, or "" if unavailable.
func NodeVersion() string {
	out, err := exec.Command("node", "--version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
