// Package scheduler owns the bounded worker pool and drives tasks through it
// for the benchmark's experiment modes (spec sections 9, 25).
package scheduler

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"bcrl/internal/metrics"
	"bcrl/internal/system"
	"bcrl/internal/workflow"
)

// Task is a single logical unit of work handed to a worker.
type Task struct {
	ID       int
	Workflow workflow.Workflow
	Created  time.Time
	Queued   time.Time
	Started  time.Time
	Finished time.Time

	done chan struct{}

	Results []workflow.Result
	Err     error
}

// Done returns a channel closed when the task completes.
func (t *Task) Done() <-chan struct{} { return t.done }

// markFinished records completion and closes the done channel once.
func (t *Task) markFinished() {
	if t.done == nil {
		return
	}
	select {
	case <-t.done:
	default:
		close(t.done)
	}
}
// Worker executes tasks. Implementations: HTTPWorker (phase 3), browser
// workers (phases 4+).
type Worker interface {
	// Run executes one task and reports its outcome.
	Run(ctx context.Context, t *Task) error
	// Close releases the worker's resources.
	Close() error
}

// Pool is a bounded set of workers that executes tasks with bounded
// concurrency. Logical concurrency (task count) is decoupled from physical
// concurrency (worker count) per spec section 25.
type Pool struct {
	workers []Worker
	jobs    chan *Task
	wg      sync.WaitGroup
	rec     *metrics.Recorder

	runCtx context.Context
	// pending counts tasks submitted but not yet finished, so Close and the
	// driver can wait for in-flight work before tearing down.
	pending sync.WaitGroup
	closed  atomic.Bool
}

// NewPool creates a pool with n workers. SetWorkers must be called before
// the first Submit.
func NewPool(n int, rec *metrics.Recorder) (*Pool, error) {
	if n < 1 {
		return nil, fmt.Errorf("worker pool size must be >= 1, got %d", n)
	}
	p := &Pool{
		jobs: make(chan *Task),
		rec:  rec,
	}
	for i := 0; i < n; i++ {
		p.wg.Add(1)
		go p.workerLoop(i)
	}
	return p, nil
}

// SetRunContext attaches the run context used to derive per-task deadlines.
// Must be called before the first Submit.
func (p *Pool) SetRunContext(ctx context.Context) {
	p.runCtx = ctx
}

func (p *Pool) workerLoop(i int) {
	defer p.wg.Done()
	selfPID := uint32(os.Getpid())
	for t := range p.jobs {
		t.Started = time.Now()
		p.rec.TaskActive()
		// Measure the peak working-set attributable to this task, summed over
		// the whole process tree (the benchmark process plus spawned
		// Chromium trees, which are separate processes). Poll during the task
		// because browsers may be launched and closed within it.
		rssBefore := system.TreeRSS(selfPID)
		peakCh := make(chan uint64, 1)
		stopPeak := make(chan struct{})
		go func() {
			peak := rssBefore
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stopPeak:
					peakCh <- peak
					return
				case <-ticker.C:
					if r := system.TreeRSS(selfPID); r > peak {
						peak = r
					}
				}
			}
		}()
		// Derive the per-task deadline from the run context so cancellation
		// (SIGINT) propagates into in-flight work. Fall back to a fresh
		// background context if no run context was set (e.g. unit tests).
		base := p.runCtx
		if base == nil {
			base = context.Background()
		}
		ctx, cancel := context.WithTimeout(base, 120*time.Second)
		err := p.workers[i].Run(ctx, t)
		cancel()
		close(stopPeak)
		peak := <-peakCh
		if peak > rssBefore {
			p.rec.RecordTaskRSSDelta(peak - rssBefore)
		} else {
			p.rec.RecordTaskRSSDelta(0)
		}
		t.Finished = time.Now()
		if err != nil {
			t.Err = err
			p.rec.TaskFailed()
		} else {
			p.rec.TaskComplete()
		}
		t.markFinished()
		p.pending.Done()
	}
}

// SetWorkers attaches the worker set the pool drives. Must be called once
// before any Submit.
func (p *Pool) SetWorkers(workers []Worker) {
	p.workers = workers
}

// WorkerCount returns the number of worker slots in the pool.
func (p *Pool) WorkerCount() int {
	return len(p.workers)
}

// Submit enqueues a task for execution. The caller should wait on t.Done().
// Returns false if the pool is closed and the task was not accepted.
func (p *Pool) Submit(t *Task) bool {
	if p.closed.Load() {
		return false
	}
	p.pending.Add(1)
	t.Queued = time.Now()
	p.rec.TaskQueued()
	p.jobs <- t
	return true
}

// Drain waits for all submitted tasks to finish.
func (p *Pool) Drain() {
	p.pending.Wait()
}

// Close stops accepting tasks, drains the queue, and stops workers.
func (p *Pool) Close() {
	if !p.closed.CompareAndSwap(false, true) {
		return
	}
	close(p.jobs)
	p.wg.Wait()
	for _, w := range p.workers {
		_ = w.Close()
	}
}
