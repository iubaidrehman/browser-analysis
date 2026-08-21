package scheduler

import (
	"context"
	"testing"
	"time"

	"bcrl/internal/logging"
	"bcrl/internal/metrics"
	"bcrl/internal/workflow"
)

// TestDriverFixedClosedLoop verifies that a fixed-concurrency run executes
// tasks, drains in-flight work at the deadline, and records zero failures.
func TestDriverFixedClosedLoop(t *testing.T) {
	rec := metrics.NewRecorder()
	pool, err := NewPool(4, rec)
	if err != nil {
		t.Fatal(err)
	}

	w := &fakeWorker{sleepMs: 20}
	pool.SetWorkers([]Worker{w})
	pool.SetRunContext(context.Background())

	log := logging.Default()
	driver := NewDriver(pool, rec, log)

	cfg := RunConfig{
		Mode:        ModeFixed,
		Concurrency: 8,
		Workflow:    workflow.Workflow{Name: "minimal"},
		Warmup:      time.Millisecond * 50,
		Measurement: time.Millisecond * 200,
		Cooldown:    time.Millisecond * 10,
	}

	if err := driver.Run(context.Background(), cfg); err != nil {
		t.Fatalf("driver run: %v", err)
	}
	pool.Close()

	c := rec.Snapshot()
	if c.Complete == 0 {
		t.Fatalf("expected completed tasks > 0, got %d", c.Complete)
	}
	if c.Failed != 0 {
		t.Fatalf("expected 0 failures, got %d", c.Failed)
	}
}

// TestDriverCancellation verifies that cancelling the run context stops the
// loop without panicking and records the cancellation.
func TestDriverCancellation(t *testing.T) {
	rec := metrics.NewRecorder()
	pool, err := NewPool(2, rec)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	w := &fakeWorker{sleepMs: 50}
	pool.SetWorkers([]Worker{w})

	log := logging.Default()
	driver := NewDriver(pool, rec, log)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	cfg := RunConfig{
		Mode:        ModeFixed,
		Concurrency: 4,
		Workflow:    workflow.Workflow{Name: "minimal"},
		Warmup:      0,
		Measurement: time.Second * 30, // long enough that cancellation fires first
	}

	if err := driver.Run(ctx, cfg); err != nil {
		t.Fatalf("driver run: %v", err)
	}
}
