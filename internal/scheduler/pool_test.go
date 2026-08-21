package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"

	"bcrl/internal/metrics"
	"bcrl/internal/workflow"
)

type fakeWorker struct {
	mu      sync.Mutex
	runs    int
	sleepMs int
	id      int // set per worker to verify each executes tasks
}

func (f *fakeWorker) Run(_ context.Context, t *Task) error {
	f.mu.Lock()
	f.runs++
	f.mu.Unlock()
	if f.sleepMs > 0 {
		time.Sleep(time.Duration(f.sleepMs) * time.Millisecond)
	}
	t.Results = append(t.Results, workflow.Result{Op: "test", Status: "ok"})
	return nil
}

func (f *fakeWorker) Close() error { return nil }

// TestPoolDistributesAcrossAllWorkers verifies that a multi-worker pool uses
// every worker — this would have caught a workers[0]-only dispatch bug.
func TestPoolDistributesAcrossAllWorkers(t *testing.T) {
	rec := metrics.NewRecorder()
	pool, err := NewPool(4, rec)
	if err != nil {
		t.Fatal(err)
	}

	workers := make([]Worker, 4)
	fakes := make([]*fakeWorker, 4)
	for i := range fakes {
		fakes[i] = &fakeWorker{id: i}
		workers[i] = fakes[i]
	}
	pool.SetWorkers(workers)

	total := 40
	var wg sync.WaitGroup
	wg.Add(total)
	for i := 0; i < total; i++ {
		task := &Task{ID: i, done: make(chan struct{})}
		go func(task *Task) {
			defer wg.Done()
			if !pool.Submit(task) {
				t.Error("submit rejected")
				return
			}
			<-task.Done()
		}(task)
	}
	wg.Wait()
	pool.Close()

	for i, f := range fakes {
		f.mu.Lock()
		runs := f.runs
		f.mu.Unlock()
		if runs == 0 {
			t.Errorf("worker %d never executed a task", i)
		}
	}
}

func TestPoolRunsTasksAndCounts(t *testing.T) {
	rec := metrics.NewRecorder()
	pool, err := NewPool(4, rec)
	if err != nil {
		t.Fatal(err)
	}

	workers := make([]Worker, 4)
	for i := range workers {
		workers[i] = &fakeWorker{sleepMs: 10}
	}
	pool.SetWorkers(workers)

	total := 20
	var wg sync.WaitGroup
	wg.Add(total)
	timedOut := make(chan int, total)
	for i := 0; i < total; i++ {
		task := &Task{ID: i, Workflow: workflow.Workflow{Name: "minimal"}, done: make(chan struct{})}
		go func(task *Task, i int) {
			defer wg.Done()
			if !pool.Submit(task) {
				timedOut <- i
				return
			}
			select {
			case <-task.Done():
			case <-time.After(5 * time.Second):
				timedOut <- i
			}
		}(task, i)
	}
	wg.Wait()
	select {
	case i := <-timedOut:
		t.Fatalf("task %d did not complete", i)
	default:
	}

	pool.Close()

	c := rec.Snapshot()
	if c.Complete != total {
		t.Fatalf("completed = %d, want %d", c.Complete, total)
	}
	if c.Failed != 0 {
		t.Fatalf("failed = %d, want 0", c.Failed)
	}
}

func TestPoolErrorCountsAsFailed(t *testing.T) {
	rec := metrics.NewRecorder()
	pool, err := NewPool(1, rec)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	errWorker := &errWorker{}
	pool.SetWorkers([]Worker{errWorker})

	task := &Task{ID: 1, done: make(chan struct{})}
	pool.Submit(task)
	<-task.Done()

	c := rec.Snapshot()
	if c.Failed != 1 {
		t.Fatalf("failed = %d, want 1", c.Failed)
	}
	if c.Complete != 0 {
		t.Fatalf("completed = %d, want 0", c.Complete)
	}
}

type errWorker struct{}

func (e *errWorker) Run(_ context.Context, _ *Task) error { return workflow.CancelledError() }
func (e *errWorker) Close() error                         { return nil }
