// Package controller orchestrates a single benchmark run end to end.
package controller

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bcrl/internal/browser"
	"bcrl/internal/cdp"
	"bcrl/internal/config"
	"bcrl/internal/contexts"
	"bcrl/internal/httpworker"
	"bcrl/internal/metrics"
	"bcrl/internal/results"
	"bcrl/internal/scheduler"
	"bcrl/internal/workflow"
	"github.com/mxschmitt/playwright-go"
)

// Options controls a run.
type Options struct {
	Config     config.Config
	Workflow   workflow.Workflow
	Repetition int
	Mode       scheduler.Mode
	ResultsDir string
	Logger     *slog.Logger
}

// Run executes a single benchmark run and persists raw results.
func Run(ctx context.Context, opts Options) (*results.Summary, error) {
	log := opts.Logger
	if log == nil {
		log = slog.Default()
	}

	cfg := opts.Config
	rec := metrics.NewRecorder()

	// Physical concurrency is decoupled from logical concurrency: workers are
	// the resource limit; tasks are the work (spec section 25). Browser
	// scenarios use browser_worker_limit; HTTP uses http_worker_limit.
	workerLimit := cfg.Concurrency.HTTPWorkerLimit
	switch cfg.Scenario {
	case "headed", "headless", "persistent-contexts", "cdp":
		workerLimit = cfg.Concurrency.BrowserWorkerLimit
	}
	pool, err := scheduler.NewPool(workerLimit, rec)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	defer pool.Close()

	var workers []scheduler.Worker

	switch cfg.Scenario {
	case "http":
		client := &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        workerLimit * 2,
				MaxIdleConnsPerHost: workerLimit,
				IdleConnTimeout:     90 * time.Second,
			},
		}
		defer client.CloseIdleConnections()
		for i := 0; i < workerLimit; i++ {
			workers = append(workers, httpworker.NewWorker(client, cfg.Target.BaseURL, rec))
		}

	case "headed", "headless":
		manager, err := browser.NewManager()
		if err != nil {
			return nil, fmt.Errorf("browser manager: %w", err)
		}
		defer manager.Close()
		headless := cfg.Scenario == "headless"
		for i := 0; i < workerLimit; i++ {
			workers = append(workers, browser.NewWorker(manager, headless, cfg.Target.BaseURL, rec))
		}

	case "persistent-contexts":
		manager, err := browser.NewManager()
		if err != nil {
			return nil, fmt.Errorf("browser manager: %w", err)
		}
		defer manager.Close()
		// Shard the requested contexts across workers: each worker's pool
		// holds a share, so 1000 contexts spread across the physical pool.
		perWorker := cfg.Contexts.Count / workerLimit
		if perWorker < 1 {
			perWorker = 1
		}
		for i := 0; i < workerLimit; i++ {
			pool, err := contexts.NewPool(manager, cfg.Browser.Headless, perWorker, rec)
			if err != nil {
				return nil, fmt.Errorf("context pool %d: %w", i, err)
			}
			workers = append(workers, contexts.NewWorker(pool, cfg.Target.BaseURL, rec))
		}

	case "cdp":
		pw, err := playwright.Run()
		if err != nil {
			return nil, fmt.Errorf("playwright run: %w", err)
		}
		defer pw.Stop()
		executable := pw.Chromium.ExecutablePath()
		perWorker := cfg.Contexts.Count / workerLimit
		if perWorker < 1 {
			perWorker = 1
		}
		for i := 0; i < workerLimit; i++ {
			w, err := cdp.New(pw, executable, cfg.Browser.Headless, perWorker, rec, cfg.Target.BaseURL)
			if err != nil {
				return nil, fmt.Errorf("cdp worker %d: %w", i, err)
			}
			workers = append(workers, w)
		}

	default:
		return nil, fmt.Errorf("scenario %q not implemented in this phase", cfg.Scenario)
	}

	pool.SetWorkers(workers)

	driver := scheduler.NewDriver(pool, rec, log)

	// Run the experiment with the configured warmup/measurement/cooldown.
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		select {
		case <-sigCh:
			log.Warn("interrupt received, cancelling benchmark")
			cancel()
		case <-runCtx.Done():
		}
	}()

	// The measurement window starts after warmup. The driver handles the full
	// window; warmup/cooldown are accounted in the metadata.
	err = driver.Run(runCtx, scheduler.RunConfig{
		Mode:        opts.Mode,
		Concurrency: cfg.Concurrency.LogicalTasks,
		Workflow:    opts.Workflow,
		Warmup:      time.Duration(cfg.Timing.WarmupSeconds) * time.Second,
		Measurement: time.Duration(cfg.Timing.MeasurementSeconds) * time.Second,
		Cooldown:    time.Duration(cfg.Timing.CooldownSeconds) * time.Second,
	})

	// On cancellation, persist partial results and report a cancellation so
	// the CLI can exit nonzero (spec section 30).
	cancelled := runCtx.Err() != nil
	if err != nil && !cancelled {
		return nil, fmt.Errorf("benchmark: %w", err)
	}

	// Persist raw results (partial on cancellation).
	dir, runID, werr := results.WriteRun(opts.ResultsDir, rec, cfg, opts.Workflow, opts.Repetition)
	if werr != nil {
		return nil, fmt.Errorf("write results: %w", werr)
	}
	log.Info("results written", "dir", dir)

	// Build and persist the summary.
	summary := results.BuildSummary(rec, cfg, opts.Workflow.Name, runID)
	if werr := results.WriteSummary(dir, summary); werr != nil {
		return nil, fmt.Errorf("write summary: %w", werr)
	}

	if cancelled {
		return summary, fmt.Errorf("benchmark cancelled")
	}
	return summary, nil
}
