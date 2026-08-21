// Package results defines the benchmark result model and raw data formats.
package results

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// RunID identifies a single benchmark run.
type RunID string

// NewRunID constructs a run identifier from a scenario, concurrency, and
// repetition number at the current UTC time.
func NewRunID(scenario string, concurrency int, repetition int) RunID {
	stamp := time.Now().UTC().Format("20060102T150405Z")
	return RunID(fmt.Sprintf("%s-%s-%d-run%02d", stamp, scenario, concurrency, repetition))
}

// Metadata captures everything needed to reproduce a run.
type Metadata struct {
	RunID       RunID       `json:"run_id"`
	Scenario    string      `json:"scenario"`
	Workflow    string      `json:"workflow"`
	Concurrency int         `json:"concurrency"`
	DurationSec int         `json:"duration_seconds"`
	WarmupSec   int         `json:"warmup_seconds"`
	CooldownSec int         `json:"cooldown_seconds"`
	Environment Environment `json:"environment"`
	Software    Software    `json:"software"`
	Config      ConfigInfo  `json:"configuration"`
	StartedAt   time.Time   `json:"started_at"`
	GitCommit   string      `json:"git_commit"`
	ConfigHash  string      `json:"config_hash"`
}

type Environment struct {
	OS           string `json:"os"`
	Kernel       string `json:"kernel"`
	CPUModel     string `json:"cpu_model"`
	CPUCores     int    `json:"cpu_cores"`
	RAMBytes     uint64 `json:"ram_bytes"`
	Architecture string `json:"architecture"`
}

type Software struct {
	GoVersion        string `json:"go_version"`
	PlaywrightVersion string `json:"playwright_version"`
	ChromiumVersion  string `json:"chromium_version"`
	NodeVersion      string `json:"node_version"`
	ReactVersion     string `json:"react_version"`
}

type ConfigInfo struct {
	BrowserMode  string `json:"browser_mode"`
	Headless     bool   `json:"headless"`
	ContextCount int    `json:"context_count"`
	WorkerCount  int    `json:"worker_count"`
}

// LatencyStats summarizes a latency distribution.
type LatencyStats struct {
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Mean   float64 `json:"mean"`
	Median float64 `json:"median"`
	P90    float64 `json:"p90"`
	P95    float64 `json:"p95"`
	P99    float64 `json:"p99"`
	StdDev float64 `json:"stddev"`
}

// Summary is the aggregate result of a single run.
type Summary struct {
	RunID         RunID         `json:"run_id"`
	TotalTasks    int           `json:"total_tasks"`
	Completed     int           `json:"completed_tasks"`
	Failed        int           `json:"failed_tasks"`
	Throughput    float64       `json:"throughput"`
	PeakRAMBytes  uint64        `json:"peak_ram_bytes"`
	AvgRAMBytes   uint64        `json:"avg_ram_bytes"`
	PeakCPU       float64       `json:"peak_cpu"`
	AvgCPU        float64       `json:"avg_cpu"`
	ProcessCount  int           `json:"process_count"`
	Latency       LatencyStats  `json:"latency"`
}

// WriteJSON writes v to a JSON file inside dir, creating dir if needed.
func WriteJSON(dir string, name string, v any) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	path := filepath.Join(dir, name)
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", name, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
