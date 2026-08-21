package contexts

import (
	"context"
	"testing"
	"time"

	"bcrl/internal/browser"
	"bcrl/internal/metrics"
	"bcrl/internal/scheduler"
	"bcrl/internal/workflow"
)

// newTestPool launches a real headless Chromium (installed via Playwright)
// with n pre-created contexts.
func newTestPool(t *testing.T, n int) (*Pool, *browser.Manager) {
	t.Helper()
	mgr, err := browser.NewManager()
	if err != nil {
		t.Fatalf("manager: %v", err)
	}
	rec := metrics.NewRecorder()
	pool, err := NewPool(mgr, true, n, rec)
	if err != nil {
		mgr.Close()
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		mgr.Close()
	})
	return pool, mgr
}

// TestPoolAcquireReleaseRoundTrip verifies acquire/release returns the same
// context to the available set.
func TestPoolAcquireReleaseRoundTrip(t *testing.T) {
	pool, _ := newTestPool(t, 2)

	c1, err := pool.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	c2, err := pool.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	if c1 == c2 {
		t.Fatal("expected distinct contexts")
	}

	pool.Release(c1)
	pool.Release(c2)

	// Pool should have 2 available again.
	c3, _ := pool.Acquire()
	c4, _ := pool.Acquire()
	if c3 == c4 {
		t.Fatal("expected distinct contexts after release")
	}
	if pool.OpenContexts() != 2 {
		t.Fatalf("open contexts = %d, want 2", pool.OpenContexts())
	}
}

// TestPoolAcquireAfterClose verifies acquire fails after close.
func TestPoolAcquireAfterClose(t *testing.T) {
	pool, mgr := newTestPool(t, 1)
	pool.Close()
	if _, err := pool.Acquire(); err == nil {
		t.Fatal("expected acquire to fail after close")
	}
	mgr.Close()
}

// TestPoolCloseIsIdempotent verifies double-close does not panic.
func TestPoolCloseIsIdempotent(t *testing.T) {
	pool, _ := newTestPool(t, 1)
	pool.Close()
	pool.Close()
}

// TestContextWorkerRunsWorkflow executes a minimal workflow against a real
// page in an acquired context and verifies success.
func TestContextWorkerRunsWorkflow(t *testing.T) {
	pool, _ := newTestPool(t, 1)
	rec := metrics.NewRecorder()
	w := NewWorker(pool, "http://localhost:65534", rec) // backend not running; minimal workflow only navigates

	task := &scheduler.Task{
		ID:       1,
		Workflow: workflow.Workflow{Name: "minimal", Steps: []workflow.Step{{Op: workflow.OpLaunch}}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := w.Run(ctx, task); err != nil {
		t.Fatalf("worker run: %v", err)
	}
}
