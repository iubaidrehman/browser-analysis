package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"bcrl/internal/config"
	"bcrl/internal/controller"
	"bcrl/internal/logging"
	"bcrl/internal/results"
	"bcrl/internal/saturation"
	"bcrl/internal/scenarios"
	"bcrl/internal/workflow"
)

// cmdRun executes a single benchmark run.
func cmdRun(args []string) error {
	opts, err := runConfigFromFlags(args)
	if err != nil {
		return err
	}
	log := logging.Default()

	summary, err := controller.Run(context.Background(), controller.Options{
		Config:      opts.Config,
		Workflow:    opts.Workflow,
		Mode:        opts.Mode,
		ResultsDir:  opts.ResultsDir,
		Repetition:  opts.Repetition,
		MetricsAddr: opts.MetricsAddr,
		Logger:      log,
	})
	if err != nil {
		return err
	}
	printSummary(summary)

	// Keep the Prometheus endpoint alive until interrupted so a scraper can
	// collect the recorded metrics.
	if opts.MetricsAddr != "" {
		fmt.Printf("serving metrics on %s/metrics (Ctrl+C to stop)\n", opts.MetricsAddr)
		select {}
	}
	return nil
}

// cmdQuick runs the quick benchmark matrix (spec section 21):
// scenarios http, headless, persistent-contexts, headed at concurrency
// 1/10/50/100.
func cmdQuick(args []string) error {
	opts, err := runConfigFromFlags(args)
	if err != nil {
		return err
	}
	cfg := opts.Config

	// Force short timing so the quick benchmark stays quick.
	cfg.Timing.WarmupSeconds = 1
	cfg.Timing.MeasurementSeconds = 5
	cfg.Timing.CooldownSeconds = 1

	scenarios := []string{"http", "headless", "persistent-contexts", "headed"}
	levels := []int{1, 10, 50, 100}
	log := logging.Default()

	// Cap browser worker limit so quick runs don't launch 100 browsers at
	// concurrency 100; physical concurrency stays bounded per spec §25.
	if cfg.Concurrency.BrowserWorkerLimit > 10 {
		cfg.Concurrency.BrowserWorkerLimit = 10
	}

	for _, scenario := range scenarios {
		for _, lvl := range levels {
			cfg.Scenario = scenario
			cfg.Concurrency.LogicalTasks = lvl
			log.Info("quick benchmark", "scenario", scenario, "concurrency", lvl)
			summary, err := controller.Run(context.Background(), controller.Options{
				Config:     cfg,
				Workflow:   opts.Workflow,
				Mode:       "fixed",
				ResultsDir: opts.ResultsDir,
				Repetition: opts.Repetition,
				Logger:     log,
			})
			if err != nil {
				return err
			}
			fmt.Printf("quick %-20s concurrency=%-3d throughput=%.1f/s completed=%d failed=%d p95=%.3fs\n",
				scenario, lvl, summary.Throughput, summary.Completed, summary.Failed, summary.Latency.P95)
		}
	}
	return nil
}

// cmdSweep runs the scenario x concurrency x repetitions matrix (spec section
// 22) with saturation detection against the lowest-concurrency baseline.
func cmdSweep(args []string) error {
	fs := flag.NewFlagSet("sweep", flag.ExitOnError)
	configPath := fs.String("config", "bench.yaml", "path to configuration file")
	scenariosArg := fs.String("scenarios", "", "comma-separated scenarios")
	concurrencyArg := fs.String("concurrency", "", "comma-separated concurrency levels")
	workflowName := fs.String("workflow", "", "workflow name (default: config)")
	reps := fs.Int("repetitions", 1, "repetitions per cell")
	resultsDir := fs.String("results", "results", "results root directory")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	if *workflowName != "" {
		cfg.Workflow.Name = *workflowName
	}
	wf, ok := workflow.Get(cfg.Workflow.Name)
	if !ok {
		return fmt.Errorf("unknown workflow %q", cfg.Workflow.Name)
	}

	scenarioList := parseList(*scenariosArg)
	if len(scenarioList) == 0 {
		scenarioList = []string{cfg.Scenario}
	}
	// Validate the scenario list up front so a typo or unimplemented scenario
	// fails fast instead of after hours of runs.
	known := map[string]bool{}
	for _, s := range scenarios.All() {
		known[s.ID] = true
	}
	for _, s := range scenarioList {
		if !known[s] {
			return fmt.Errorf("unknown or unimplemented scenario %q", s)
		}
	}

	levels := parseList(*concurrencyArg)
	if len(levels) == 0 {
		levels = []string{"1", "5", "10", "25", "50", "100", "250", "500", "750", "1000"}
	}

	log := logging.Default()
	th := saturation.DefaultThresholds()
	var sweepRec []sweepCellResult

	for _, scenario := range scenarioList {
		cfg.Scenario = scenario

		// Baseline: the lowest concurrency level, averaged across repetitions
		// so a single cold-start run cannot corrupt the ratios.
		baseLvl, err := strconv.Atoi(levels[0])
		if err != nil {
			return fmt.Errorf("invalid concurrency %q", levels[0])
		}
		cfg.Concurrency.LogicalTasks = baseLvl
		var baseAcc results.Summary
		baseCount := 0
		for r := 1; r <= *reps; r++ {
			summary, err := runSweepCell(cfg, wf, *resultsDir, scenario, baseLvl, r, log)
			if err != nil {
				return err
			}
			accumulate(&baseAcc, summary)
			baseCount++
		}
		baseAcc.Failed /= baseCount
		baseAcc.TotalTasks /= baseCount
		baseAcc.Latency.P95 /= float64(baseCount)
		baseAcc.Latency.P99 /= float64(baseCount)
		baseAcc.AvgCPU /= float64(baseCount)
		baseAcc.AvgRAMBytes /= uint64(baseCount)
		baseline := &baseAcc
		fmt.Printf("sweep %-20s concurrency=%-4d baseline p95=%.3fs cpu=%.1f%% (avg of %d)\n",
			scenario, baseLvl, baseline.Latency.P95, baseline.AvgCPU, baseCount)
		sweepRec = append(sweepRec, sweepCellResult{
			Scenario: scenario, Concurrency: baseLvl, Baseline: true,
			P95: baseline.Latency.P95, CPU: baseline.AvgCPU,
		})

		for _, lvlStr := range levels[1:] {
			lvl, err := strconv.Atoi(lvlStr)
			if err != nil {
				return fmt.Errorf("invalid concurrency %q", lvlStr)
			}
			cfg.Concurrency.LogicalTasks = lvl
			for r := 1; r <= *reps; r++ {
				summary, err := runSweepCell(cfg, wf, *resultsDir, scenario, lvl, r, log)
				if err != nil {
					return err
				}
				ev := saturation.Evaluate(summary, baseline, th)
				status := "ok"
				if ev.Saturated {
					status = "SATURATED: " + strings.Join(ev.Violations, ",")
				}
				fmt.Printf("sweep %-20s concurrency=%-4d p95=%.3fs failed=%d rate=%.2f%% -> %s\n",
					scenario, lvl, summary.Latency.P95, summary.Failed,
					float64(summary.Failed)/float64(max(summary.TotalTasks, 1))*100, status)
				sweepRec = append(sweepRec, sweepCellResult{
					Scenario: scenario, Concurrency: lvl, Rep: r,
					P95: summary.Latency.P95, P99: summary.Latency.P99,
					Failed: summary.Failed, Total: summary.TotalTasks,
					CPU: summary.AvgCPU, Saturated: ev.Saturated,
					Violations: ev.Violations,
				})
			}
		}
	}

	// Persist the sweep evaluation for later analysis.
	dir := filepath.Join(*resultsDir, "sweeps")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(sweepRec, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, time.Now().UTC().Format("20060102T150405Z")+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	fmt.Printf("sweep evaluation written to %s\n", path)
	return nil
}

// runSweepCell runs one sweep cell.
func runSweepCell(cfg config.Config, wf workflow.Workflow, resultsDir, scenario string, lvl, rep int, log *slog.Logger) (*results.Summary, error) {
	cfg.Scenario = scenario
	cfg.Concurrency.LogicalTasks = lvl
	log.Info("sweep cell", "scenario", scenario, "concurrency", lvl, "rep", rep)
	return controller.Run(context.Background(), controller.Options{
		Config:     cfg,
		Workflow:   wf,
		Mode:       "fixed",
		ResultsDir: resultsDir,
		Repetition: rep,
		Logger:     log,
	})
}

// accumulate folds a summary into an accumulator for baseline averaging.
func accumulate(acc, s *results.Summary) {
	acc.Failed += s.Failed
	acc.TotalTasks += s.TotalTasks
	acc.Latency.P95 += s.Latency.P95
	acc.Latency.P99 += s.Latency.P99
	acc.AvgCPU += s.AvgCPU
	acc.AvgRAMBytes += s.AvgRAMBytes
}

// sweepCellResult is one persisted sweep row.
type sweepCellResult struct {
	Scenario    string   `json:"scenario"`
	Concurrency int      `json:"concurrency"`
	Rep         int      `json:"rep,omitempty"`
	Baseline    bool     `json:"baseline,omitempty"`
	P95         float64  `json:"p95"`
	P99         float64  `json:"p99,omitempty"`
	Failed      int      `json:"failed,omitempty"`
	Total       int      `json:"total,omitempty"`
	CPU         float64  `json:"cpu_percent,omitempty"`
	Saturated   bool     `json:"saturated,omitempty"`
	Violations  []string `json:"violations,omitempty"`
}

func parseList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// cmdSummarize prints a human summary of all runs under results/raw.
func cmdSummarize(args []string) error {
	resultsDir := "results"
	if len(args) > 0 && args[0] != "" {
		resultsDir = args[0]
	}
	runs, err := listRuns(resultsDir)
	if err != nil {
		return err
	}
	if len(runs) == 0 {
		fmt.Println("No measurement available.")
		return nil
	}
	fmt.Printf("%-30s %-14s %-10s %-8s %-8s %-8s %-8s\n",
		"RUN_ID", "SCENARIO", "WORKFLOW", "THROUGHPUT", "COMPLETED", "FAILED", "P95")
	for _, r := range runs {
		fmt.Printf("%-30s %-14s %-10s %-8.1f %-8d %-8d %-8.4f\n",
			r.ID, r.Scenario, r.Workflow, r.Throughput, r.Completed, r.Failed, r.P95)
	}
	return nil
}

func cmdStatus() error {
	dir := filepath.Join("results", "raw")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("no runs yet")
			return nil
		}
		return err
	}
	fmt.Printf("%d run(s) recorded in %s\n", len(entries), dir)
	for _, e := range entries {
		fmt.Println(" ", e.Name())
	}
	return nil
}

// cmdCleanup removes leftover benchmark state (results dir is preserved).
func cmdCleanup() error {
	fmt.Println("No leftover state to clean up.")
	return nil
}

func printSummary(s *results.Summary) {
	fmt.Printf("\nRun summary\n")
	fmt.Printf("  total tasks:   %d\n", s.TotalTasks)
	fmt.Printf("  completed:     %d\n", s.Completed)
	fmt.Printf("  failed:        %d\n", s.Failed)
	fmt.Printf("  throughput:    %.1f/s\n", s.Throughput)
	fmt.Printf("  latency:       mean %.3fs p50 %.3fs p95 %.3fs p99 %.3fs\n",
		s.Latency.Mean, s.Latency.Median, s.Latency.P95, s.Latency.P99)
}

// runEntry is one row of the summarize table.
type runEntry struct {
	ID         string
	Scenario   string
	Workflow   string
	Throughput float64
	Completed  int
	Failed     int
	P95        float64
}

// readJSONFile decodes a JSON file into v.
func readJSONFile(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// listRuns reads each run directory's summary.json.
func listRuns(resultsDir string) ([]runEntry, error) {
	rawDir := filepath.Join(resultsDir, "raw")
	entries, err := os.ReadDir(rawDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var runs []runEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		var s results.Summary
		if err := readJSONFile(filepath.Join(rawDir, e.Name(), "summary.json"), &s); err != nil {
			continue // skip unreadable runs
		}
		runs = append(runs, runEntry{
			ID:         string(s.RunID),
			Scenario:   s.Scenario,
			Workflow:   s.Workflow,
			Throughput: s.Throughput,
			Completed:  s.Completed,
			Failed:     s.Failed,
			P95:        s.Latency.P95,
		})
	}
	return runs, nil
}
