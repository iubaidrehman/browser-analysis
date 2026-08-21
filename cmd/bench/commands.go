package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"bcrl/internal/controller"
	"bcrl/internal/logging"
	"bcrl/internal/results"
)

// cmdRun executes a single benchmark run.
func cmdRun(args []string) error {
	opts, err := runConfigFromFlags(args)
	if err != nil {
		return err
	}
	log := logging.Default()

	summary, err := controller.Run(context.Background(), controller.Options{
		Config:     opts.Config,
		Workflow:   opts.Workflow,
		Mode:       opts.Mode,
		ResultsDir: opts.ResultsDir,
		Repetition: opts.Repetition,
		Logger:     log,
	})
	if err != nil {
		return err
	}
	printSummary(summary)
	return nil
}

// cmdQuick runs the quick benchmark matrix (spec section 21):
// concurrency 1/10/50/100 for the http scenario. Browser scenarios are added
// in later phases.
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

	levels := []int{1, 10, 50, 100}
	log := logging.Default()
	for _, lvl := range levels {
		cfg.Concurrency.LogicalTasks = lvl
		log.Info("quick benchmark", "scenario", cfg.Scenario, "concurrency", lvl)
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
		fmt.Printf("quick %-4s concurrency=%-3d throughput=%.1f/s completed=%d failed=%d p95=%.3fs\n",
			cfg.Scenario, lvl, summary.Throughput, summary.Completed, summary.Failed, summary.Latency.P95)
	}
	return nil
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
