// Package results defines the benchmark result model, raw data files, and
// summary computation (spec sections 15-18).
package results

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"bcrl/internal/buildinfo"
	"bcrl/internal/config"
	"bcrl/internal/metrics"
	"bcrl/internal/stats"
	"bcrl/internal/workflow"
)

// RunID identifies a single benchmark run.
type RunID string

// NewRunID constructs a run identifier from a scenario, concurrency, and
// repetition number at the current UTC time.
func NewRunID(scenario string, concurrency int, repetition int) RunID {
	stamp := time.Now().UTC().Format("20060102T150405Z")
	return RunID(fmt.Sprintf("%s-%s-%d-run%02d", stamp, scenario, concurrency, repetition))
}

// Metadata captures everything needed to reproduce a run (spec section 16).
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

// Environment describes the host machine.
type Environment struct {
	OS           string `json:"os"`
	Kernel       string `json:"kernel"`
	CPUModel     string `json:"cpu_model"`
	CPUCores     int    `json:"cpu_cores"`
	RAMBytes     uint64 `json:"ram_bytes"`
	Architecture string `json:"architecture"`
}

// Software records tool versions.
type Software struct {
	GoVersion         string `json:"go_version"`
	PlaywrightVersion string `json:"playwright_version"`
	ChromiumVersion   string `json:"chromium_version"`
	NodeVersion       string `json:"node_version"`
	ReactVersion      string `json:"react_version"`
}

// ConfigInfo is the effective run configuration.
type ConfigInfo struct {
	BrowserMode  string `json:"browser_mode"`
	Headless     bool   `json:"headless"`
	ContextCount int    `json:"context_count"`
	WorkerCount  int    `json:"worker_count"`
}

// Summary is the aggregate result of a single run (spec section 17).
type Summary struct {
	RunID          RunID           `json:"run_id"`
	Scenario       string          `json:"scenario"`
	Workflow       string          `json:"workflow"`
	Concurrency    int             `json:"concurrency"`
	TotalTasks     int             `json:"total_tasks"`
	Completed      int             `json:"completed_tasks"`
	Failed         int             `json:"failed_tasks"`
	Throughput     float64         `json:"throughput"`
	PeakRAMBytes   uint64          `json:"peak_ram_bytes"`
	AvgRAMBytes    uint64          `json:"avg_ram_bytes"`
	PeakCPU        float64         `json:"peak_cpu"`
	AvgCPU         float64         `json:"avg_cpu"`
	ProcessCount   int             `json:"process_count"`
	Latency          stats.Summary   `json:"latency"`
	BrowserLaunch    stats.Summary   `json:"browser_launch"`
	ContextCreation  stats.Summary   `json:"context_creation"`
	CDPConnect       stats.Summary   `json:"cdp_connect"`
	TaskRSSBytes     stats.Summary   `json:"task_rss_bytes"`
	Escalations      int             `json:"escalations,omitempty"`
	HTTPStepTotal    float64         `json:"http_step_total_seconds,omitempty"`
	BrowserStepTotal float64         `json:"browser_step_total_seconds,omitempty"`
	Failures         map[string]int  `json:"failures,omitempty"`
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

// WriteRun persists all raw data files for a run and returns the directory
// and the run ID.
func WriteRun(resultsDir string, rec *metrics.Recorder, cfg config.Config, wf workflow.Workflow, repetition int, mode string) (string, RunID, error) {
	runID := NewRunID(cfg.Scenario, cfg.Concurrency.LogicalTasks, repetition)
	dir := filepath.Join(resultsDir, "raw", string(runID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", fmt.Errorf("create run dir: %w", err)
	}

	meta := Metadata{
		RunID:       runID,
		Scenario:    cfg.Scenario,
		Workflow:    wf.Name,
		Concurrency: cfg.Concurrency.LogicalTasks,
		DurationSec: cfg.Timing.MeasurementSeconds,
		WarmupSec:   cfg.Timing.WarmupSeconds,
		CooldownSec: cfg.Timing.CooldownSeconds,
		Environment: environmentInfo(),
		Software:    softwareInfo(),
		Config: ConfigInfo{
			BrowserMode:  cfg.Browser.Engine,
			Headless:     cfg.Browser.Headless,
			ContextCount: cfg.Contexts.Count,
			WorkerCount:  workerCountFor(cfg),
		},
		StartedAt:  time.Now().UTC(),
		GitCommit:  buildinfo.GitCommit(),
		ConfigHash: buildinfo.ConfigHash(cfg, mode),
	}
	if err := WriteJSON(dir, "metadata.json", meta); err != nil {
		return "", "", err
	}

	// task_metrics.csv — task counters snapshot.
	if err := writeCountersCSV(dir, rec); err != nil {
		return "", "", err
	}

	// browser_metrics.csv — per-browser launch latencies (scenarios A/B/C/D).
	if err := writeBrowserCSV(dir, rec); err != nil {
		return "", "", err
	}

	// hybrid_metrics.csv — HTTP vs browser step time splits (scenario F).
	if err := writeHybridCSV(dir, rec); err != nil {
		return "", "", err
	}

	// task_rss_metrics.csv — per-task working-set deltas (bytes).
	if err := writeRSSMetricsCSV(dir, rec); err != nil {
		return "", "", err
	}

	return dir, runID, nil
}

// writeRSSMetricsCSV writes per-task RSS deltas, one row per task.
func writeRSSMetricsCSV(dir string, rec *metrics.Recorder) error {
	deltas := rec.RSSDeltas()
	path := filepath.Join(dir, "task_rss_metrics.csv")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write([]string{"task_rss_delta_bytes"}); err != nil {
		return err
	}
	for _, d := range deltas {
		if err := w.Write([]string{strconv.FormatFloat(d, 'f', 0, 64)}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

// writeHybridCSV writes hybrid transport step durations, one row per step.
func writeHybridCSV(dir string, rec *metrics.Recorder) error {
	httpSteps, browserSteps := rec.TransportLatencies()
	path := filepath.Join(dir, "hybrid_metrics.csv")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write([]string{"http_step_seconds", "browser_step_seconds"}); err != nil {
		return err
	}
	rows := len(httpSteps)
	if len(browserSteps) > rows {
		rows = len(browserSteps)
	}
	for i := 0; i < rows; i++ {
		hs := ""
		if i < len(httpSteps) {
			hs = strconv.FormatFloat(httpSteps[i], 'f', 9, 64)
		}
		bs := ""
		if i < len(browserSteps) {
			bs = strconv.FormatFloat(browserSteps[i], 'f', 9, 64)
		}
		if err := w.Write([]string{hs, bs}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

// writeBrowserCSV writes browser launch, context creation, and CDP connect
// latencies.
func writeBrowserCSV(dir string, rec *metrics.Recorder) error {
	_, _, _, launches, contextCreations, cdpConnects := rec.Latencies()
	path := filepath.Join(dir, "browser_metrics.csv")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write([]string{"launch_latency_seconds", "context_creation_seconds", "cdp_connect_seconds"}); err != nil {
		return err
	}
	rows := len(launches)
	for _, s := range [][]float64{contextCreations, cdpConnects} {
		if len(s) > rows {
			rows = len(s)
		}
	}
	for i := 0; i < rows; i++ {
		launch := ""
		if i < len(launches) {
			launch = strconv.FormatFloat(launches[i], 'f', 9, 64)
		}
		ctx := ""
		if i < len(contextCreations) {
			ctx = strconv.FormatFloat(contextCreations[i], 'f', 9, 64)
		}
		cdp := ""
		if i < len(cdpConnects) {
			cdp = strconv.FormatFloat(cdpConnects[i], 'f', 9, 64)
		}
		if err := w.Write([]string{launch, ctx, cdp}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

// writeCountersCSV writes the task/request counters as a single-row CSV so
// there is always a valid task_metrics.csv even before phase 9 telemetry.
func writeCountersCSV(dir string, rec *metrics.Recorder) error {
	c := rec.Snapshot()
	path := filepath.Join(dir, "task_metrics.csv")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	_ = w.Write([]string{
		"tasks_created", "tasks_queued", "tasks_active", "tasks_completed",
		"tasks_failed", "tasks_cancelled", "retries", "requests_ok",
		"requests_failed", "ws_events", "workflow_failures",
	})
	_ = w.Write([]string{
		itoa(c.Created), itoa(c.Queued), itoa(c.Active), itoa(c.Complete),
		itoa(c.Failed), itoa(c.Canceled), itoa(c.Retries), itoa(c.RequestsOK),
		itoa(c.RequestsFailed), itoa(c.WSEvents), itoa(c.WorkflowFailed),
	})
	w.Flush()
	return w.Error()
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

// workerCountFor returns the physical worker limit for the scenario: browser
// scenarios use browser_worker_limit, HTTP uses http_worker_limit.
func workerCountFor(cfg config.Config) int {
	switch cfg.Scenario {
	case "headed", "headless", "persistent-contexts", "cdp", "hybrid":
		return cfg.Concurrency.BrowserWorkerLimit
	default:
		return cfg.Concurrency.HTTPWorkerLimit
	}
}

// BuildSummary computes the aggregate summary from recorded metrics.
func BuildSummary(rec *metrics.Recorder, cfg config.Config, workflowName string, runID RunID) *Summary {
	workflowLat, _, _, browserLaunchLat, contextCreationLat, cdpConnectLat := rec.Latencies()
	httpSteps, browserSteps := rec.TransportLatencies()
	rssDeltas := rec.RSSDeltas()
	dist := stats.Summarize(workflowLat)
	launchDist := stats.Summarize(browserLaunchLat)
	contextDist := stats.Summarize(contextCreationLat)
	cdpDist := stats.Summarize(cdpConnectLat)
	rssDist := stats.Summarize(rssDeltas)

	c := rec.Snapshot()
	total := c.Created
	if total == 0 {
		total = c.Complete + c.Failed
	}
	measurementSec := float64(cfg.Timing.MeasurementSeconds)
	// Use the actual measurement wall time when the driver recorded it (the
	// drain after the deadline can extend the window beyond the configured
	// duration), falling back to the configured value.
	if el := rec.MeasurementElapsed(); el > 0 {
		measurementSec = el
	}
	throughput := float64(c.Complete) / measurementSec
	if measurementSec == 0 {
		throughput = 0
	}

	return &Summary{
		RunID:          runID,
		Scenario:       cfg.Scenario,
		Workflow:       workflowName,
		Concurrency:    cfg.Concurrency.LogicalTasks,
		TotalTasks:     total,
		Completed:      c.Complete,
		Failed:         c.Failed,
		Throughput:     throughput,
		Latency:        dist,
		BrowserLaunch:  launchDist,
		ContextCreation: contextDist,
		CDPConnect:     cdpDist,
		TaskRSSBytes:   rssDist,
		Escalations:    c.Escalations,
		HTTPStepTotal:  sum(httpSteps),
		BrowserStepTotal: sum(browserSteps),
		Failures:       rec.Failures(),
	}
}

func sum(vals []float64) float64 {
	var s float64
	for _, v := range vals {
		s += v
	}
	return s
}

// WriteSummary persists the summary.json for a run directory.
func WriteSummary(dir string, s *Summary) error {
	return WriteJSON(dir, "summary.json", s)
}

// environmentInfo gathers host facts. Platform-specific collectors fill the
// gaps (cpu model, ram) in phase 9/10.
func environmentInfo() Environment {
	cores := runtime.NumCPU()
	return Environment{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		CPUCores:     cores,
	}
}

// softwareInfo gathers Go/Node versions. Playwright/Chromium/React versions
// are filled in by later phases.
func softwareInfo() Software {
	return Software{
		GoVersion:   buildinfo.GoVersion(),
		NodeVersion: buildinfo.NodeVersion(),
	}
}
