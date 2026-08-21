// Package httpworker implements the lightweight HTTP worker (scenario E).
package httpworker

import (
	"context"
	"fmt"
	"net/http"

	"bcrl/internal/metrics"
	"bcrl/internal/scheduler"
	"bcrl/internal/workflow"
)

// Worker executes workflows through HTTP requests only, with no browser.
type Worker struct {
	exec  *workflow.HTTPExecutor
	rec   *metrics.Recorder
}

// NewWorker builds an HTTP worker sharing the given client and executor.
func NewWorker(client *http.Client, baseURL string, rec *metrics.Recorder) *Worker {
	return &Worker{
		exec: workflow.NewHTTPExecutor(client, baseURL),
		rec:  rec,
	}
}

// Run executes the task's workflow over HTTP.
func (w *Worker) Run(ctx context.Context, t *scheduler.Task) error {
	results, dur, err := w.exec.Execute(ctx, t.Workflow)
	t.Results = results

	// Record per-step and request latencies.
	for _, r := range results {
		if r.Status == "ok" && r.Duration > 0 {
			w.rec.RecordStep(r.Duration)
		}
		if r.Request != nil {
			w.rec.RecordRequest(r.Request.Duration, r.Status == "ok")
		}
		if r.Error != nil {
			// Classify into the spec §29 taxonomy; keep the raw HTTP message
			// as a secondary breakdown when present.
			w.rec.RecordFailure(workflow.ClassifyError(r.Error))
		}
	}

	if err != nil {
		w.rec.WorkflowFailed()
		return fmt.Errorf("workflow %s: %w", t.Workflow.Name, err)
	}
	w.rec.RecordWorkflow(dur)
	return nil
}

// Close is a no-op for HTTP workers.
func (w *Worker) Close() error { return nil }
