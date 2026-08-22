package contexts

import (
	"context"
	"fmt"
	"time"

	"bcrl/internal/browser"
	"bcrl/internal/metrics"
	"bcrl/internal/scheduler"
	"bcrl/internal/workflow"
)

// Worker implements scheduler.Worker for scenario C: each worker owns one
// persistent Chromium + a context pool, and each task acquires a context,
// runs the workflow in a page, then releases the context.
type Worker struct {
	pool    *Pool
	baseURL string
	rec     *metrics.Recorder
}

// NewWorker builds a persistent-context worker.
func NewWorker(pool *Pool, baseURL string, rec *metrics.Recorder) *Worker {
	return &Worker{pool: pool, baseURL: baseURL, rec: rec}
}

// Run acquires a context, executes the workflow, and releases it. On error
// the context is closed (not returned to the pool) so a poisoned context
// never serves the next task.
func (w *Worker) Run(ctx context.Context, t *scheduler.Task) error {
	bctx, err := w.pool.Acquire()
	if err != nil {
		return fmt.Errorf("acquire context: %w", err)
	}

	page, err := bctx.NewPage()
	if err != nil {
		w.pool.Release(bctx)
		return fmt.Errorf("new page: %w", err)
	}
	w.rec.RecordLifecycle(metrics.LifecycleEvent{Type: metrics.EvPageCreateCompleted, At: time.Now(), TaskID: t.ID})
	defer page.Close()

	exec := browser.NewPageExecutor(page)
	results, dur, err := exec.Execute(ctx, w.baseURL, t.Workflow)
	t.Results = results

	for _, r := range results {
		if r.Status == "ok" && r.Duration > 0 {
			w.rec.RecordStep(r.Duration)
		}
		if r.Error != nil {
			w.rec.RecordFailure(workflow.ClassifyError(r.Error))
		}
	}

	if err != nil {
		// Poisoned context: close it, don't return it to the pool.
		_ = bctx.Close()
		w.pool.Release(bctx)
		w.rec.WorkflowFailed()
		return fmt.Errorf("workflow %s: %w", t.Workflow.Name, err)
	}
	w.pool.Release(bctx)
	w.rec.RecordWorkflow(dur)
	return nil
}

// Close releases the persistent browser.
func (w *Worker) Close() error {
	w.pool.Close()
	return nil
}
