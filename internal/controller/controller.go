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

	"bcrl/internal/config"
	"bcrl/internal/httpworker"
	"bcrl/internal/metrics"
	"bcrl/internal/results"
	"bcrl/internal/scheduler"
	"bcrl/internal/workflow"
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

	// Build the worker pool. Physical concurrency is decoupled from logical
	// concurrency: workers are the resource limit; tasks are the work.
	pool, err := scheduler.NewPool(cfg.Concurrency.HTTPWorkerLimit, rec)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	client := &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        cfg.Concurrency.HTTPWorkerLimit * 2,
			MaxIdleConnsPerHost: cfg.Concurrency.HTTPWorkerLimit,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	workers := make([]scheduler.Worker, cfg.Concurrency.HTTPWorkerLimit)
	for i := range workers {
		workers[i] = httpworker.NewWorker(client, cfg.Target.BaseURL, rec)
	}
	pool.SetWorkers(workers)
	defer pool.Close()
	defer client.CloseIdleConnections()

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
