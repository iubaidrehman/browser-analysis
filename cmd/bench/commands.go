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
	maxRSSGB := fs.Float64("max-rss-gb", 0, "stop sweep if previous level peak RAM exceeds this (GB); 0 disables")
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
		baseAcc.TaskRSSBytes.Mean /= float64(baseCount)
		baseline := &baseAcc
		fmt.Printf("sweep %-20s concurrency=%-4d baseline p95=%.3fs cpu=%.1f%% rss=%.0fMB (avg of %d)\n",
			scenario, baseLvl, baseline.Latency.P95, baseline.AvgCPU,
			baseline.TaskRSSBytes.Mean/1048576, baseCount)
		sweepRec = append(sweepRec, sweepCellResult{
			Scenario: scenario, Concurrency: baseLvl, Baseline: true,
			P95: baseline.Latency.P95, CPU: baseline.AvgCPU,
			RSSMeanMB: baseline.TaskRSSBytes.Mean / 1048576,
		})

		prevLevel := baseLvl
		prevPeakRAM := uint64(0)
		for _, lvlStr := range levels[1:] {
			lvl, err := strconv.Atoi(lvlStr)
			if err != nil {
				return fmt.Errorf("invalid concurrency %q", lvlStr)
			}
			// Adaptive safety (milestone section 17): before escalating to a
			// higher concurrency, inspect the previous level's peak memory.
			// If it exceeds the configured ceiling, stop the sweep, persist
			// everything so far, and mark the next configuration not-run.
			if *maxRSSGB > 0 && prevPeakRAM > 0 && float64(prevPeakRAM) > *maxRSSGB*1073741824 {
				fmt.Printf("safety: peak RAM %.1f GB at concurrency %d exceeds ceiling %.1f GB; stopping sweep\n",
					float64(prevPeakRAM)/1073741824, prevLevel, *maxRSSGB)
				for _, nl := range append([]string{lvlStr}, levels[levelIdx(levels, lvlStr)+1:]...) {
					sweepRec = append(sweepRec, sweepCellResult{
						Scenario: scenario, Concurrency: atoiOr(nl, 0), NotRun: true,
					})
				}
				goto sweepDone
			}
			cfg.Concurrency.LogicalTasks = lvl
			levelPeak := uint64(0)
			for r := 1; r <= *reps; r++ {
				summary, err := runSweepCell(cfg, wf, *resultsDir, scenario, lvl, r, log)
				if err != nil {
					return err
				}
				if summary.PeakRAMBytes > levelPeak {
					levelPeak = summary.PeakRAMBytes
				}
				ev := saturation.Evaluate(summary, baseline, th)
				status := "ok"
				if ev.Saturated {
					status = "SATURATED: " + strings.Join(ev.Violations, ",")
				}
				fmt.Printf("sweep %-20s concurrency=%-4d p95=%.3fs failed=%d rss=%.0fMB rate=%.2f%% -> %s\n",
					scenario, lvl, summary.Latency.P95, summary.Failed,
					summary.TaskRSSBytes.Mean/1048576,
					float64(summary.Failed)/float64(max(summary.TotalTasks, 1))*100, status)
				sweepRec = append(sweepRec, sweepCellResult{
					Scenario: scenario, Concurrency: lvl, Rep: r,
					P95: summary.Latency.P95, P99: summary.Latency.P99,
					Failed: summary.Failed, Total: summary.TotalTasks,
					CPU: summary.AvgCPU, Saturated: ev.Saturated,
					RSSMeanMB: summary.TaskRSSBytes.Mean / 1048576,
					RSSP95MB:  summary.TaskRSSBytes.P95 / 1048576,
					Violations: ev.Violations,
				})
			}
			prevLevel = lvl
			prevPeakRAM = levelPeak
		}
	}
sweepDone:

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

	// Milestone section 18: write scaling + resource summaries.
	if err := writeSweepSummaries(*resultsDir); err != nil {
		return fmt.Errorf("write sweep summaries: %w", err)
	}
	return nil
}

// writeSweepSummaries aggregates each run's resource_summary.json into
// scaling-summary.csv and resource-summary.csv under results/summaries.
func writeSweepSummaries(resultsDir string) error {
	rawDir := filepath.Join(resultsDir, "raw")
	entries, err := os.ReadDir(rawDir)
	if err != nil {
		return err
	}
	var resRows []results.ResourceRow
	var scaleRows []results.ScalingRow
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		var rs results.ResourceSummary
		if err := readJSONFile(filepath.Join(rawDir, e.Name(), "resource_summary.json"), &rs); err != nil {
			continue // pre-milestone runs lack the file
		}
		var s results.Summary
		if err := readJSONFile(filepath.Join(rawDir, e.Name(), "summary.json"), &s); err != nil {
			continue
		}
		resRows = append(resRows, results.ResourceRow{
			RunID: e.Name(), Scenario: s.Scenario, Concurrency: s.Concurrency,
			TotalRSSMeanMB:     float64(rs.TotalRSS.Mean) / 1048576,
			TotalRSSP95MB:      float64(rs.TotalRSS.P95) / 1048576,
			TotalRSSPeakMB:     float64(rs.TotalRSS.Peak) / 1048576,
			BrowserRSSMeanMB:   float64(rs.BrowserRSS.Mean) / 1048576,
			BrowserRSSP95MB:    float64(rs.BrowserRSS.P95) / 1048576,
			TaskRSSMeanMB:      float64(rs.TaskRSS.Mean) / 1048576,
			BenchmarkCPUMean:   rs.BenchmarkCPU.Mean,
			ProcessMean:        float64(rs.TotalProcesses.Mean),
			BrowserProcessMean: float64(rs.BrowserProcesses.Mean),
			Browsers:           rs.Browsers, Contexts: rs.Contexts, Pages: rs.Pages,
		})
		scaleRows = append(scaleRows, results.ScalingRow{
			Concurrency: s.Concurrency, Throughput: s.Throughput,
			P95: s.Latency.P95, P99: s.Latency.P99, CPU: s.AvgCPU,
			RSSMeanMB:         float64(rs.TotalRSS.Mean) / 1048576,
			BrowserRSSMeanMB:  float64(rs.BrowserRSS.Mean) / 1048576,
			ProcessMean:       float64(rs.TotalProcesses.Mean),
			BrowserProcessMean: float64(rs.BrowserProcesses.Mean),
			Contexts:          rs.Contexts,
			FailureRate:       failureRate(&s),
			Saturated:         rs.ArchitectureRSSDelta > 0 && s.AvgCPU > 90,
		})
	}
	summaryDir := filepath.Join(resultsDir, "summaries")
	if err := os.MkdirAll(summaryDir, 0o755); err != nil {
		return err
	}
	if err := results.WriteScalingCSV(summaryDir, scaleRows); err != nil {
		return err
	}
	if err := results.WriteResourceCSV(summaryDir, resRows); err != nil {
		return err
	}
	fmt.Printf("summaries written: %s/resource-summary.csv, %s/scaling-summary.csv\n", summaryDir, summaryDir)
	return nil
}

func failureRate(s *results.Summary) float64 {
	if s.TotalTasks == 0 {
		return 0
	}
	return float64(s.Failed) / float64(s.TotalTasks)
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
	acc.TaskRSSBytes.Mean += s.TaskRSSBytes.Mean
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
	RSSMeanMB   float64  `json:"rss_mean_mb,omitempty"`
	RSSP95MB    float64  `json:"rss_p95_mb,omitempty"`
	NotRun      bool     `json:"not_run,omitempty"`
	Saturated   bool     `json:"saturated,omitempty"`
	Violations  []string `json:"violations,omitempty"`
}

func levelIdx(levels []string, s string) int {
	for i, l := range levels {
		if l == s {
			return i
		}
	}
	return len(levels) - 1
}

func atoiOr(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
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

// cmdResources prints the resource accounting for one run (milestone section 15).
func cmdResources(args []string) error {
	fs := flag.NewFlagSet("resources", flag.ExitOnError)
	runID := fs.String("run", "", "run id (directory name under results/raw)")
	resultsDir := fs.String("results", "results", "results root directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *runID == "" {
		return fmt.Errorf("--run is required")
	}

	var rs results.ResourceSummary
	dir := filepath.Join(*resultsDir, "raw", *runID)
	if err := readJSONFile(filepath.Join(dir, "resource_summary.json"), &rs); err != nil {
		return fmt.Errorf("read resource summary for %s: %w", *runID, err)
	}

	fmt.Printf("Resources for run %s\n", *runID)
	fmt.Println("  MEMORY")
	fmt.Printf("    baseline total:        %d MB\n", rs.BaselineTotalRSS/1048576)
	fmt.Printf("    architecture delta:    %d MB\n", rs.ArchitectureRSSDelta/1048576)
	fmt.Printf("    total mean/p95/peak:   %d / %d / %d MB\n",
		rs.TotalRSS.Mean/1048576, rs.TotalRSS.P95/1048576, rs.TotalRSS.Peak/1048576)
	fmt.Printf("    browser mean/p95/peak: %d / %d / %d MB\n",
		rs.BrowserRSS.Mean/1048576, rs.BrowserRSS.P95/1048576, rs.BrowserRSS.Peak/1048576)
	fmt.Printf("    controller mean:       %d MB\n", rs.ControllerRSS.Mean/1048576)
	fmt.Printf("    target mean:           %d MB\n", rs.TargetRSS.Mean/1048576)
	fmt.Println("  CPU (percent of one core)")
	fmt.Printf("    benchmark mean/p95:    %.1f / %.1f\n", rs.BenchmarkCPU.Mean, rs.BenchmarkCPU.P95)
	fmt.Printf("    browser mean/p95:      %.1f / %.1f\n", rs.BrowserCPU.Mean, rs.BrowserCPU.P95)
	fmt.Println("  PROCESSES")
	fmt.Printf("    total mean/peak:       %d / %d\n", rs.TotalProcesses.Mean, rs.TotalProcesses.Peak)
	fmt.Printf("    browser mean/peak:     %d / %d\n", rs.BrowserProcesses.Mean, rs.BrowserProcesses.Peak)
	fmt.Printf("    renderer mean:         %d\n", rs.RendererProcesses.Mean)
	fmt.Printf("    utility mean:          %d\n", rs.UtilityProcesses.Mean)
	fmt.Printf("    gpu mean:              %d\n", rs.GPUProcesses.Mean)
	fmt.Println("  CONCURRENCY")
	fmt.Printf("    logical tasks:         %d\n", rs.Workers)
	fmt.Printf("    browsers:              %d\n", rs.Browsers)
	fmt.Printf("    contexts:              %d\n", rs.Contexts)
	fmt.Printf("    pages:                 %d\n", rs.Pages)
	fmt.Println("  DERIVED")
	fmt.Printf("    memory per task:       %d MB\n", rs.MemoryPerLogicalTask/1048576)
	fmt.Printf("    throughput per cpu%%:   %.3f\n", rs.ThroughputPerCPU)
	fmt.Printf("    throughput per GB:     %.3f\n", rs.ThroughputPerGBRSS)
	return nil
}

// cmdTopology prints the process topology of one run (milestone section 15).
func cmdTopology(args []string) error {
	fs := flag.NewFlagSet("topology", flag.ExitOnError)
	runID := fs.String("run", "", "run id (directory name under results/raw)")
	resultsDir := fs.String("results", "results", "results root directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *runID == "" {
		return fmt.Errorf("--run is required")
	}

	var rs results.ResourceSummary
	dir := filepath.Join(*resultsDir, "raw", *runID)
	if err := readJSONFile(filepath.Join(dir, "resource_summary.json"), &rs); err != nil {
		return fmt.Errorf("read resource summary for %s: %w", *runID, err)
	}

	fmt.Printf("Topology for run %s\n", *runID)
	fmt.Printf("  total processes:     mean %d, peak %d\n", rs.TotalProcesses.Mean, rs.TotalProcesses.Peak)
	fmt.Printf("  browser processes:   mean %d, peak %d\n", rs.BrowserProcesses.Mean, rs.BrowserProcesses.Peak)
	fmt.Printf("  renderer processes:  mean %d, peak %d\n", rs.RendererProcesses.Mean, rs.RendererProcesses.Peak)
	fmt.Printf("  utility processes:   mean %d, peak %d\n", rs.UtilityProcesses.Mean, rs.UtilityProcesses.Peak)
	fmt.Printf("  gpu processes:       mean %d, peak %d\n", rs.GPUProcesses.Mean, rs.GPUProcesses.Peak)
	fmt.Printf("  controller processes: mean %d, peak %d\n", rs.ControllerProcesses.Mean, rs.ControllerProcesses.Peak)
	fmt.Printf("  target processes:    mean %d, peak %d\n", rs.TargetProcesses.Mean, rs.TargetProcesses.Peak)
	return nil
}

// cmdAnalyzeRun prints the full per-run analysis (latency, throughput, memory).
func cmdAnalyzeRun(args []string) error {
	fs := flag.NewFlagSet("analyze-run", flag.ExitOnError)
	runID := fs.String("run", "", "run id (directory name under results/raw)")
	resultsDir := fs.String("results", "results", "results root directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *runID == "" {
		return fmt.Errorf("--run is required")
	}
	dir := filepath.Join(*resultsDir, "raw", *runID)

	var s results.Summary
	if err := readJSONFile(filepath.Join(dir, "summary.json"), &s); err != nil {
		return fmt.Errorf("read summary for %s: %w", *runID, err)
	}
	fmt.Printf("Run %s (%s / %s / concurrency %d)\n", s.RunID, s.Scenario, s.Workflow, s.Concurrency)
	fmt.Printf("  throughput:  %.1f/s   completed=%d failed=%d\n", s.Throughput, s.Completed, s.Failed)
	fmt.Printf("  latency:     mean=%.4fs p50=%.4fs p95=%.4fs p99=%.4fs (n=%d)\n",
		s.Latency.Mean, s.Latency.Median, s.Latency.P95, s.Latency.P99, s.Latency.Count)
	fmt.Printf("  task rss:    mean=%.0fMB p95=%.0fMB max=%.0fMB\n",
		s.TaskRSSBytes.Mean/1048576, s.TaskRSSBytes.P95/1048576, s.TaskRSSBytes.Max/1048576)
	return cmdResources([]string{"--run", *runID, "--results", *resultsDir})
}

// cmdAnalyzeSweep prints per-scenario saturation and scaling from a sweep file.
func cmdAnalyzeSweep(args []string) error {
	fs := flag.NewFlagSet("analyze-sweep", flag.ExitOnError)
	sweep := fs.String("sweep", "", "sweep file name under results/sweeps")
	resultsDir := fs.String("results", "results", "results root directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *sweep == "" {
		return fmt.Errorf("--sweep is required")
	}
	var cells []sweepCellResult
	if err := readJSONFile(filepath.Join(*resultsDir, "sweeps", *sweep), &cells); err != nil {
		return err
	}
	fmt.Printf("Sweep %s: %d cells\n", *sweep, len(cells))
	byScenario := map[string][]sweepCellResult{}
	for _, c := range cells {
		byScenario[c.Scenario] = append(byScenario[c.Scenario], c)
	}
	for scenario, sc := range byScenario {
		fmt.Printf("  %s: %d cells, %d saturated\n", scenario, len(sc), countSaturated(sc))
	}
	return nil
}

func countSaturated(cells []sweepCellResult) int {
	n := 0
	for _, c := range cells {
		if c.Saturated {
			n++
		}
	}
	return n
}

// cmdReport writes a markdown report from the latest sweep evaluations and
// run summaries (spec section 35).
func cmdReport(args []string) error {
	resultsDir := "results"
	if len(args) > 0 && args[0] != "" {
		resultsDir = args[0]
	}

	var b strings.Builder
	b.WriteString("# BCRL Benchmark Report\n\n")
	b.WriteString("Generated by `bench report`. All raw data lives in `results/raw`.\n\n")

	// Latest sweep evaluation, if any.
	sweepsDir := filepath.Join(resultsDir, "sweeps")
	if entries, err := os.ReadDir(sweepsDir); err == nil && len(entries) > 0 {
		latest := entries[len(entries)-1]
		var cells []sweepCellResult
		if err := readJSONFile(filepath.Join(sweepsDir, latest.Name()), &cells); err == nil {
			b.WriteString("## Sweep: " + strings.TrimSuffix(latest.Name(), ".json") + "\n\n")
			b.WriteString("| scenario | concurrency | rep | p95 (s) | p99 (s) | failed | cpu% | rss mean (MB) | rss p95 (MB) | saturated |\n")
			b.WriteString("|---|---|---|---|---|---|---|---|---|---|\n")
			for _, c := range cells {
				sat := ""
				if c.Saturated {
					sat = "**" + strings.Join(c.Violations, ",") + "**"
				}
				// Older sweep files predate the RSS fields; render them as
				// "n/a" instead of misleading zeros.
				rssMean, rssP95 := "n/a", "n/a"
				if c.RSSMeanMB > 0 {
					rssMean = fmt.Sprintf("%.0f", c.RSSMeanMB)
					rssP95 = fmt.Sprintf("%.0f", c.RSSP95MB)
				}
				// A baseline cell (Rep 0) is a single averaged sample; P99
				// from one observation is not meaningful (milestone section
				// 11), so render N/A rather than a fabricated 0.
				p99 := fmt.Sprintf("%.4f", c.P99)
				if c.Rep == 0 && c.P99 == 0 {
					p99 = "N/A"
				}
				b.WriteString(fmt.Sprintf("| %s | %d | %d | %.4f | %s | %d | %.1f | %s | %s | %s |\n",
					c.Scenario, c.Concurrency, c.Rep, c.P95, p99, c.Failed, c.CPU,
					rssMean, rssP95, sat))
			}
			b.WriteString("\n")
		}
	}

	// Summaries table across scenarios.
	runs, err := listRuns(resultsDir)
	if err != nil {
		return err
	}
	if len(runs) > 0 {
		b.WriteString("## Runs\n\n")
		b.WriteString("| run | scenario | workflow | throughput/s | completed | failed | p95 (s) |\n")
		b.WriteString("|---|---|---|---|---|---|---|\n")
		for _, r := range runs {
			b.WriteString(fmt.Sprintf("| %s | %s | %s | %.1f | %d | %d | %.4f |\n",
				r.ID, r.Scenario, r.Workflow, r.Throughput, r.Completed, r.Failed, r.P95))
		}
		b.WriteString("\n")
	} else {
		b.WriteString("No measurement available.\n\n")
	}

	// Note on interpretation.
	b.WriteString("## Interpretation\n\n")
	b.WriteString("This report presents raw measurements only. The benchmark makes no\n")
	b.WriteString("claim that any architecture is faster; conclusions require statistical\n")
	b.WriteString("analysis of the raw data (see `docs/methodology.md`).\n")

	out := filepath.Join(resultsDir, "report.md")
	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		return err
	}
	fmt.Printf("report written to %s\n", out)
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
