// Package config loads and validates benchmark configuration.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the root benchmark configuration.
type Config struct {
	Scenario    string      `yaml:"scenario" json:"scenario"`
	Concurrency Concurrency `yaml:"concurrency" json:"concurrency"`
	Browser     Browser     `yaml:"browser" json:"browser"`
	Contexts    Contexts    `yaml:"contexts" json:"contexts"`
	Workflow    Workflow    `yaml:"workflow" json:"workflow"`
	Timing      Timing      `yaml:"timing" json:"timing"`
	Target      Target      `yaml:"target" json:"target"`
	Telemetry   Telemetry   `yaml:"telemetry" json:"telemetry"`
	Hybrid      Hybrid      `yaml:"hybrid" json:"hybrid"`
}

// Hybrid controls scenario F's escalation policy (spec section 28).
type Hybrid struct {
	// Escalation selects which operations route to the browser. The only
	// implemented policy is "workflow": browser-only ops (localStorage,
	// IndexedDB, execute-js, WebSocket) plus navigation escalate; API ops
	// stay on HTTP.
	Escalation string `yaml:"escalation" json:"escalation"`
}

type Concurrency struct {
	LogicalTasks      int `yaml:"logical_tasks" json:"logical_tasks"`
	BrowserWorkerLimit int `yaml:"browser_worker_limit" json:"browser_worker_limit"`
	HTTPWorkerLimit    int `yaml:"http_worker_limit" json:"http_worker_limit"`
}

type Browser struct {
	Engine       string `yaml:"engine" json:"engine"`
	Headless     bool   `yaml:"headless" json:"headless"`
	ReuseBrowser bool   `yaml:"reuse_browser" json:"reuse_browser"`
}

type Contexts struct {
	Count int `yaml:"count" json:"count"`
}

type Workflow struct {
	Name string `yaml:"name" json:"name"`
}

type Timing struct {
	WarmupSeconds      int `yaml:"warmup_seconds" json:"warmup_seconds"`
	MeasurementSeconds int `yaml:"measurement_seconds" json:"measurement_seconds"`
	CooldownSeconds    int `yaml:"cooldown_seconds" json:"cooldown_seconds"`
}

type Target struct {
	BaseURL string `yaml:"base_url" json:"base_url"`
}

type Telemetry struct {
	IntervalSeconds int `yaml:"interval_seconds" json:"interval_seconds"`
}

// Defaults returns a configuration with safe baseline values.
func Defaults() Config {
	return Config{
		Scenario: "http",
		Concurrency: Concurrency{
			LogicalTasks:       100,
			BrowserWorkerLimit: 50,
			HTTPWorkerLimit:    100,
		},
		Browser: Browser{
			Engine:       "chromium",
			Headless:     true,
			ReuseBrowser: true,
		},
		Contexts: Contexts{Count: 100},
		Workflow: Workflow{Name: "complex"},
		Timing: Timing{
			WarmupSeconds:      30,
			MeasurementSeconds: 180,
			CooldownSeconds:    30,
		},
		Target: Target{BaseURL: "http://localhost:8080"},
		Telemetry: Telemetry{
			IntervalSeconds: 1,
		},
		Hybrid: Hybrid{Escalation: "workflow"},
	}
}

// Load reads a YAML configuration file into a Config, applying defaults for
// any value left unset in the file.
func Load(path string) (Config, error) {
	cfg := Defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("validate config %s: %w", path, err)
	}

	return cfg, nil
}

// Validate checks that the configuration is internally consistent.
func (c Config) Validate() error {
	if c.Scenario == "" {
		return fmt.Errorf("scenario must not be empty")
	}
	if c.Concurrency.LogicalTasks < 1 {
		return fmt.Errorf("concurrency.logical_tasks must be >= 1")
	}
	if c.Concurrency.BrowserWorkerLimit < 1 {
		return fmt.Errorf("concurrency.browser_worker_limit must be >= 1")
	}
	if c.Concurrency.HTTPWorkerLimit < 1 {
		return fmt.Errorf("concurrency.http_worker_limit must be >= 1")
	}
	if c.Timing.WarmupSeconds < 0 || c.Timing.MeasurementSeconds < 1 || c.Timing.CooldownSeconds < 0 {
		return fmt.Errorf("timing values must be non-negative (measurement_seconds >= 1)")
	}
	if c.Target.BaseURL == "" {
		return fmt.Errorf("target.base_url must not be empty")
	}
	if c.Telemetry.IntervalSeconds < 1 {
		return fmt.Errorf("telemetry.interval_seconds must be >= 1")
	}
	if c.Hybrid.Escalation != "" && c.Hybrid.Escalation != "workflow" {
		return fmt.Errorf("hybrid.escalation must be %q, got %q", "workflow", c.Hybrid.Escalation)
	}
	return nil
}

// MeasurementDuration returns the primary measurement window as a duration.
func (c Config) MeasurementDuration() time.Duration {
	return time.Duration(c.Timing.MeasurementSeconds) * time.Second
}
