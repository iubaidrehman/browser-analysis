package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultsValidate(t *testing.T) {
	if err := Defaults().Validate(); err != nil {
		t.Fatalf("defaults should validate: %v", err)
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bench.yaml")
	data := []byte("scenario: persistent-contexts\nconcurrency:\n  logical_tasks: 25\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Scenario != "persistent-contexts" {
		t.Fatalf("scenario = %q, want persistent-contexts", cfg.Scenario)
	}
	if cfg.Concurrency.LogicalTasks != 25 {
		t.Fatalf("logical_tasks = %d, want 25", cfg.Concurrency.LogicalTasks)
	}
	// Untouched field should retain its default.
	if cfg.Target.BaseURL != "http://localhost:8080" {
		t.Fatalf("base_url = %q, want default", cfg.Target.BaseURL)
	}
}

func TestLoadInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bench.yaml")
	data := []byte("scenario: \"\"\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected validation error for empty scenario")
	}
}
