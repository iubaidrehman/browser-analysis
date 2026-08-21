package hybrid

import (
	"context"
	"fmt"
	"net/http"

	"bcrl/internal/browser"
	"bcrl/internal/contexts"
	"bcrl/internal/metrics"
	"bcrl/internal/scheduler"
	"bcrl/internal/workflow"
)

// Worker implements scenario F: each worker owns a persistent browser
// context pool plus an HTTP client. Per task it acquires a context, creates
// a page, and routes workflow steps across HTTP and the browser.
type Worker struct {
	httpExec *workflow.HTTPExecutor
	pool     *contexts.Pool
	baseURL  string
	rec      *metrics.Recorder
	policy   EscalationPolicy
}

// NewWorker builds a hybrid worker over the given context pool and HTTP
// client.
func NewWorker(policy EscalationPolicy, pool *contexts.Pool, client *http.Client, baseURL string, rec *metrics.Recorder) *Worker {
	return &Worker{
		httpExec: workflow.NewHTTPExecutor(client, baseURL),
		pool:     pool,
		baseURL:  baseURL,
		rec:      rec,
		policy:   policy,
	}
}

// Run executes the workflow across transports.
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
	defer page.Close()

	exec := NewExecutor(w.policy, w.httpExec, browser.NewPageExecutor(page), w.baseURL)
	results, dur, escalations, err := exec.Execute(ctx, t.Workflow)
	t.Results = results

	// Step latencies, split by transport (spec section 28: time spent HTTP
	// vs browser).
	for _, r := range results {
		if r.Status == "ok" && r.Duration > 0 {
			if w.policy.BrowserOnly(r.Op) {
				w.rec.RecordBrowserStep(r.Duration)
			} else {
				w.rec.RecordHTTPStep(r.Duration)
			}
		}
		if r.Error != nil {
			w.rec.RecordFailure(workflow.ClassifyError(r.Error))
		}
	}

	if err != nil {
		_ = bctx.Close()
		w.pool.Release(bctx)
		w.rec.WorkflowFailed()
		return fmt.Errorf("workflow %s: %w", t.Workflow.Name, err)
	}

	// Escalation telemetry is gated on workflow success so partial/failed
	// workflows don't inflate the count.
	for i := 0; i < escalations; i++ {
		w.rec.RecordEscalation()
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
