package cdp

import (
	"context"
	"testing"
	"time"

	"bcrl/internal/metrics"
	"bcrl/internal/scheduler"
	"bcrl/internal/workflow"
	"github.com/mxschmitt/playwright-go"
)

// TestCDPWorkerRunsWorkflow spawns a real headless Chromium, connects over
// CDP, and runs a launch-only workflow.
func TestCDPWorkerRunsWorkflow(t *testing.T) {
	pw, err := playwright.Run()
	if err != nil {
		t.Fatalf("playwright run: %v", err)
	}
	defer pw.Stop()

	rec := metrics.NewRecorder()
	exe := pw.Chromium.ExecutablePath()
	w, err := New(pw, exe, true, 1, rec, "http://localhost:65534")
	if err != nil {
		t.Fatalf("cdp worker: %v", err)
	}
	defer w.Close()

	task := &scheduler.Task{
		ID:       1,
		Workflow: workflow.Workflow{Name: "minimal", Steps: []workflow.Step{{Op: workflow.OpLaunch}}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := w.Run(ctx, task); err != nil {
		t.Fatalf("worker run: %v", err)
	}
}
