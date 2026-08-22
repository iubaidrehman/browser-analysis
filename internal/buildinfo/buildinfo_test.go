package buildinfo

import (
	"strings"
	"testing"

	"bcrl/internal/config"
)

func TestGitCommitNonEmpty(t *testing.T) {
	c := GitCommit()
	if c == "" {
		t.Skip("not a git repository")
	}
	if !strings.HasPrefix(c, "8fa164f") && len(c) < 40 && !strings.Contains(c, "-dirty") {
		t.Fatalf("unexpected git commit %q", c)
	}
}

func TestConfigHashDeterministic(t *testing.T) {
	cfg := config.Defaults()
	h1 := ConfigHash(cfg, "fixed")
	h2 := ConfigHash(cfg, "fixed")
	if h1 != h2 {
		t.Fatalf("hash not deterministic: %q vs %q", h1, h2)
	}
	if len(h1) != 16 {
		t.Fatalf("hash length = %d, want 16", len(h1))
	}
}

func TestConfigHashModeSensitive(t *testing.T) {
	cfg := config.Defaults()
	h1 := ConfigHash(cfg, "fixed")
	h2 := ConfigHash(cfg, "ramp")
	if h1 == h2 {
		t.Fatal("hash should differ by experiment mode")
	}
}
