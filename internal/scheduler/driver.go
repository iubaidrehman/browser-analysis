package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"bcrl/internal/metrics"
	"bcrl/internal/workflow"
)

// Mode is an experiment mode (spec section 9).
type Mode string

const (
	ModeFixed  Mode = "fixed"
	ModeRamp   Mode = "ramp"
	ModeStep   Mode = "step"
	ModeSteady Mode = "steady"
	ModeSpike  Mode = "spike"
)

// RunConfig describes a single benchmark run.
type RunConfig struct {
	Mode        Mode
	Concurrency int // logical tasks in flight (FIXED/STEADY/SPIKE)
	Workflow    workflow.Workflow
	Warmup      time.Duration
	Measurement time.Duration
	Cooldown    time.Duration
}

// Driver executes a RunConfig against a pool.
type Driver struct {
	pool *Pool
	rec  *metrics.Recorder
	log  *slog.Logger
}

// NewDriver builds a driver for the given pool.
func NewDriver(pool *Pool, rec *metrics.Recorder, log *slog.Logger) *Driver {
	return &Driver{pool: pool, rec: rec, log: log}
}

// Run executes the run configuration and blocks until it completes.
func (d *Driver) Run(ctx context.Context, cfg RunConfig) error {
	d.pool.SetRunContext(ctx)
	switch cfg.Mode {
	case ModeFixed, ModeSteady:
		return d.runWithWarmup(ctx, cfg, func(dur time.Duration) error {
			return d.runFixed(ctx, cfg, cfg.Concurrency, dur)
		})
	case ModeRamp:
		return d.runWithWarmup(ctx, cfg, func(dur time.Duration) error {
			return d.runRamp(ctx, cfg)
		})
	case ModeStep:
		return d.runWithWarmup(ctx, cfg, func(dur time.Duration) error {
			return d.runStep(ctx, cfg)
		})
	case ModeSpike:
		return d.runWithWarmup(ctx, cfg, func(dur time.Duration) error {
			return d.runSpike(ctx, cfg)
		})
	default:
		return fmt.Errorf("unknown mode %q", cfg.Mode)
	}
}

// runWithWarmup executes the warmup phase, then resets the recorder at the
// measurement boundary so warmup data never enters the primary measurements
// (spec section 10).
func (d *Driver) runWithWarmup(ctx context.Context, cfg RunConfig, run func(time.Duration) error) error {
	if cfg.Warmup > 0 {
		d.log.Info("warmup", "seconds", cfg.Warmup.Seconds())
		if err := run(cfg.Warmup); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		d.rec.Reset()
	}
	d.log.Info("measurement", "seconds", cfg.Measurement.Seconds())
	return run(cfg.Measurement)
}

// newTask builds a fresh task for the closed loop.
func (d *Driver) newTask(id int, cfg RunConfig) *Task {
	return &Task{ID: id, Workflow: cfg.Workflow, done: make(chan struct{})}
}

// runFixed keeps n logical tasks in flight continuously for the given
// duration. Each task re-submits after completion, so the in-flight count
// stays at n (a closed loop). Physical concurrency is bounded by the pool's
// worker count; logical concurrency larger than that simply saturates the
// pool (spec section 25). After the deadline it drains in-flight tasks so no
// completed work is dropped from the summary.
func (d *Driver) runFixed(ctx context.Context, cfg RunConfig, n int, duration time.Duration) error {
	if n < 1 {
		return fmt.Errorf("concurrency must be >= 1, got %d", n)
	}
	deadline := time.Now().Add(duration)

	// Bound the number of in-flight logical tasks by the number of workers.
	// Extra logical tasks queue up naturally on the jobs channel.
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					d.rec.TaskCancelled()
					return
				default:
				}
				if time.Now().After(deadline) {
					return
				}
				// A fresh task per iteration: reusing the same Task across
				// iterations would leave its done channel closed, causing the
				// pool's markFinished to panic on the second close.
				t := d.newTask(i, cfg)
				t.Created = time.Now()
				if !d.pool.Submit(t) {
					return // pool closed; stop submitting
				}
				select {
				case <-t.Done():
				case <-ctx.Done():
					d.rec.TaskCancelled()
					return
				}
			}
		}(i)
	}
	wg.Wait()
	// Drain in-flight tasks so their results are included in the summary.
	d.pool.Drain()
	return nil
}

// runRamp divides the total measurement window across the concurrency levels,
// holding each level for an equal share.
func (d *Driver) runRamp(ctx context.Context, cfg RunConfig) error {
	levels := rampLevels(cfg.Concurrency)
	hold := cfg.Measurement / time.Duration(len(levels))
	if hold < time.Second {
		hold = time.Second
	}
	for _, lvl := range levels {
		d.log.Info("ramp level", "concurrency", lvl, "hold", hold)
		if err := d.runFixed(ctx, cfg, lvl, hold); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	return nil
}

// runStep increases concurrency every stepInterval, with each level held for
// stepInterval seconds summing to the measurement window.
func (d *Driver) runStep(ctx context.Context, cfg RunConfig) error {
	levels := rampLevels(cfg.Concurrency)
	stepInterval := cfg.Measurement / time.Duration(len(levels))
	if stepInterval < time.Second {
		stepInterval = time.Second
	}
	for _, lvl := range levels {
		d.log.Info("step level", "concurrency", lvl, "hold", stepInterval)
		if err := d.runFixed(ctx, cfg, lvl, stepInterval); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	return nil
}

// runSpike ramps quickly to the configured concurrency and holds it. The
// ramp-up is bounded by the pool's accept rate, which under saturation is a
// near-instant burst.
func (d *Driver) runSpike(ctx context.Context, cfg RunConfig) error {
	d.log.Info("spike", "target", cfg.Concurrency)
	return d.runFixed(ctx, cfg, cfg.Concurrency, cfg.Measurement)
}

// rampLevels returns the concurrency levels up to max (spec section 8).
func rampLevels(max int) []int {
	all := []int{1, 5, 10, 25, 50, 100, 250, 500, 750, 1000}
	var out []int
	for _, lvl := range all {
		if lvl > max {
			break
		}
		out = append(out, lvl)
	}
	if len(out) == 0 {
		out = []int{1}
	}
	return out
}
