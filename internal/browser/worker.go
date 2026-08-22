package browser

import (
	"context"
	"fmt"
	"sync"
	"time"

	"bcrl/internal/metrics"
	"bcrl/internal/scheduler"
	"bcrl/internal/workflow"
	"github.com/mxschmitt/playwright-go"
)

// Manager owns the shared Playwright driver. Chromium launches are performed
// directly on the shared driver, which playwright-go makes concurrency-safe
// (apiZone keyed per goroutine, atomic call ids); no mutex is held across
// Launch so browser startup latency is never serialized by the manager.
type Manager struct {
	pw *playwright.Playwright

	mu       sync.Mutex
	started  bool
	startDur time.Duration
	startErr error
}

// NewManager starts (or connects to) Playwright and returns a manager.
func NewManager() (*Manager, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("playwright run: %w", err)
	}
	return &Manager{pw: pw}, nil
}

// StartBrowser launches a fresh Chromium instance and times the launch.
func (m *Manager) StartBrowser(headless bool) (playwright.Browser, time.Duration, error) {
	start := time.Now()
	browser, err := m.pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(headless),
	})
	if err != nil {
		m.mu.Lock()
		m.started = true
		m.startErr = err
		m.mu.Unlock()
		return nil, 0, fmt.Errorf("launch chromium: %w", err)
	}
	dur := time.Since(start)
	m.mu.Lock()
	m.started = true
	m.startDur = dur
	m.mu.Unlock()
	return browser, dur, nil
}

// Close shuts down the Playwright driver.
func (m *Manager) Close() error {
	if m.pw != nil {
		return m.pw.Stop()
	}
	return nil
}

// Worker implements scheduler.Worker for scenarios A (headed) and B
// (headless): each task launches an independent Chromium, opens a page,
// runs the workflow, and closes the browser.
type Worker struct {
	manager  *Manager
	headless bool
	baseURL  string
	rec      *metrics.Recorder
}

// NewWorker builds a per-task-browser worker.
func NewWorker(manager *Manager, headless bool, baseURL string, rec *metrics.Recorder) *Worker {
	return &Worker{manager: manager, headless: headless, baseURL: baseURL, rec: rec}
}

// Run launches a browser, executes the workflow in a page, and closes it.
// Cleanup is unconditional via defers on the worker's single goroutine, so
// browsers close on success, workflow error, and cancellation.
func (w *Worker) Run(ctx context.Context, t *scheduler.Task) error {
	browser, launchDur, err := w.manager.StartBrowser(w.headless)
	if err != nil {
		return err
	}
	defer browser.Close()

	// Browser launch is a measured lifecycle event; keep it separate from
	// workflow step latencies.
	w.rec.RecordBrowserLaunch(launchDur)
	w.rec.RecordLifecycle(metrics.LifecycleEvent{Type: metrics.EvBrowserLaunchCompleted, At: time.Now(), TaskID: t.ID})

	page, err := browser.NewPage()
	if err != nil {
		return fmt.Errorf("new page: %w", err)
	}
	w.rec.RecordLifecycle(metrics.LifecycleEvent{Type: metrics.EvPageCreateCompleted, At: time.Now(), TaskID: t.ID})
	defer page.Close()

	exec := NewPageExecutor(page)
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
		w.rec.WorkflowFailed()
		return fmt.Errorf("workflow %s: %w", t.Workflow.Name, err)
	}
	w.rec.RecordWorkflow(dur)
	return nil
}

// Close is a no-op; the manager is closed by the controller.
func (w *Worker) Close() error { return nil }
