package cdp

import (
	"context"
	"fmt"
	"time"

	"bcrl/internal/browser"
	"bcrl/internal/contexts"
	"bcrl/internal/metrics"
	"bcrl/internal/scheduler"
	"bcrl/internal/workflow"
	"github.com/mxschmitt/playwright-go"
)

// Worker implements scenario D: it spawns Chromium raw, connects over CDP,
// and runs each task in a context acquired from a persistent pool. Startup,
// connect, and context creation latencies are measured separately.
type Worker struct {
	launcher *Launcher
	browser  playwright.Browser
	pool     *contexts.Pool
	baseURL  string
	rec      *metrics.Recorder
}

// New launches Chromium, connects via CDP, and builds the context pool.
func New(pw *playwright.Playwright, executable string, headless bool, contextsPerWorker int, rec *metrics.Recorder, baseURL string) (*Worker, error) {
	launcher, port, spawnDur, err := Launch(executable, headless)
	if err != nil {
		return nil, err
	}
	// The raw spawn (process start → DevToolsActivePort) is the browser
	// startup measurement for scenario D, comparable to A/B's launch.
	rec.RecordBrowserLaunch(spawnDur)

	connectStart := time.Now()
	connBrowser, closeConn, err := ConnectOverCDP(pw, port)
	if err != nil {
		_ = launcher.Kill()
		return nil, err
	}
	connectDur := time.Since(connectStart)
	rec.RecordCDPConnect(connectDur)

	pool, err := contexts.NewPoolWithBrowser(connBrowser, contextsPerWorker, rec)
	if err != nil {
		// The pool may have partially created contexts; kill the raw
		// Chromium explicitly — pool.Close on a CDP-connected browser only
		// drops the connection and does not stop the spawned process.
		_ = closeConn()
		_ = launcher.Kill()
		return nil, err
	}

	return &Worker{
		launcher: launcher,
		browser:  connBrowser,
		pool:     pool,
		baseURL:  baseURL,
		rec:      rec,
	}, nil
}

// Run acquires a context, runs the workflow in a page, releases the context.
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
		_ = bctx.Close()
		w.pool.Release(bctx)
		w.rec.WorkflowFailed()
		return fmt.Errorf("workflow %s: %w", t.Workflow.Name, err)
	}
	w.pool.Release(bctx)
	w.rec.RecordWorkflow(dur)
	return nil
}

// Close tears down the pool, CDP connection, and the spawned Chromium.
func (w *Worker) Close() error {
	w.pool.Close()
	if w.browser != nil {
		_ = w.browser.Close()
	}
	if w.launcher != nil {
		return w.launcher.Kill()
	}
	return nil
}
