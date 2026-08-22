// Command bench is the Browser Concurrency Research Lab controller CLI.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"bcrl/internal/config"
	"bcrl/internal/scheduler"
	"bcrl/internal/scenarios"
	"bcrl/internal/workflow"
)

const version = "0.2.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "version":
		fmt.Printf("bench %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
	case "validate":
		err = cmdValidate(os.Args[2:])
	case "list-scenarios":
		err = cmdListScenarios()
	case "run":
		err = cmdRun(os.Args[2:])
	case "quick":
		err = cmdQuick(os.Args[2:])
	case "sweep":
		err = cmdSweep(os.Args[2:])
	case "summarize":
		err = cmdSummarize(os.Args[2:])
	case "report":
		err = cmdReport(os.Args[2:])
	case "resources":
		err = cmdResources(os.Args[2:])
	case "topology":
		err = cmdTopology(os.Args[2:])
	case "analyze-run":
		err = cmdAnalyzeRun(os.Args[2:])
	case "analyze-sweep":
		err = cmdAnalyzeSweep(os.Args[2:])
	case "status":
		err = cmdStatus()
	case "cleanup":
		err = cmdCleanup()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println("Usage: bench <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  version          print version information")
	fmt.Println("  validate         validate configuration and environment")
	fmt.Println("  list-scenarios   list available benchmark scenarios")
	fmt.Println("  run              run a single benchmark")
	fmt.Println("  quick            run the quick benchmark matrix")
	fmt.Println("  sweep            run a scenario/concurrency matrix")
	fmt.Println("  summarize        summarize raw results")
	fmt.Println("  report           write a markdown report from sweep results")
	fmt.Println("  resources        show per-run resource accounting")
	fmt.Println("  topology         show per-run process topology")
	fmt.Println("  analyze-run      deep-dive analysis of a single run")
	fmt.Println("  analyze-sweep    deep-dive analysis of a sweep")
	fmt.Println("  status           show current benchmark state")
	fmt.Println("  cleanup          remove leftover state")
}

func cmdListScenarios() error {
	fmt.Printf("%-20s  %s\n", "SCENARIO", "DESCRIPTION")
	for _, s := range scenarios.All() {
		status := "planned"
		if s.Implemented {
			status = "implemented"
		}
		fmt.Printf("%-20s  %s (%s)\n", s.ID, s.Description, status)
	}
	return nil
}

func cmdValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	configPath := fs.String("config", "bench.yaml", "path to configuration file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	fmt.Println("Benchmark environment:")
	fmt.Printf("  OS:        %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  Go:        %s\n", runtime.Version())
	fmt.Printf("  Cores:     %d\n", runtime.NumCPU())
	if _, err := config.Load(*configPath); err != nil {
		fmt.Printf("  Config:    %s (error: %v)\n", *configPath, err)
		return err
	}
	fmt.Printf("  Config:    %s (valid)\n", *configPath)
	return nil
}

// runOptions carries parsed CLI flags plus the loaded config.
type runOptions struct {
	Config      config.Config
	Workflow    workflow.Workflow
	Mode        scheduler.Mode
	ResultsDir  string
	Repetition  int
	MetricsAddr string
}

// runConfigFromFlags parses shared run flags and loads the YAML config,
// overlaying CLI flags on top.
func runConfigFromFlags(args []string) (runOptions, error) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	configPath := fs.String("config", "bench.yaml", "path to configuration file")
	scenario := fs.String("scenario", "", "override scenario")
	workflowName := fs.String("workflow", "", "override workflow name")
	concurrency := fs.Int("concurrency", 0, "override logical task concurrency")
	duration := fs.Int("duration", 0, "override measurement duration in seconds")
	warmup := fs.Int("warmup", 0, "override warmup in seconds")
	cooldown := fs.Int("cooldown", 0, "override cooldown in seconds")
	mode := fs.String("mode", "fixed", "experiment mode: fixed|ramp|step|steady|spike")
	resultsDir := fs.String("results", "results", "results root directory")
	repeat := fs.Int("repeat", 1, "repetition number for run_id")
	metricsAddr := fs.String("metrics-addr", "", "optional Prometheus /metrics listen address")
	if err := fs.Parse(args); err != nil {
		return runOptions{}, err
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return runOptions{}, err
	}
	if *scenario != "" {
		cfg.Scenario = *scenario
	}
	if *workflowName != "" {
		cfg.Workflow.Name = *workflowName
	}
	if *concurrency > 0 {
		cfg.Concurrency.LogicalTasks = *concurrency
	}
	if *duration > 0 {
		cfg.Timing.MeasurementSeconds = *duration
	}
	if *warmup > 0 {
		cfg.Timing.WarmupSeconds = *warmup
	}
	if *cooldown > 0 {
		cfg.Timing.CooldownSeconds = *cooldown
	}

	wf, ok := workflow.Get(cfg.Workflow.Name)
	if !ok {
		return runOptions{}, fmt.Errorf("unknown workflow %q", cfg.Workflow.Name)
	}

	if err := cfg.Validate(); err != nil {
		return runOptions{}, err
	}

	return runOptions{
		Config:      cfg,
		Workflow:    wf,
		Mode:        scheduler.Mode(*mode),
		ResultsDir:  *resultsDir,
		Repetition:  *repeat,
		MetricsAddr: *metricsAddr,
	}, nil
}
