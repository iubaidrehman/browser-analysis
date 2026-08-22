package accounting

import (
	"sync"

	"bcrl/internal/process"
)

// Aggregator consumes process snapshots, classifies them, and accumulates the
// resource series for one run. It captures a baseline before the workload
// begins.
type Aggregator struct {
	ownership *process.Ownership
	mu        sync.Mutex
	baseline  Sample
	haveBase  bool
	samples   []Sample
}

// NewAggregator builds an aggregator for the given controller and target pids.
func NewAggregator(controllerPID, targetPID uint32) *Aggregator {
	return &Aggregator{
		ownership: &process.Ownership{ControllerPID: controllerPID, TargetPID: targetPID},
	}
}

// Classify assigns architecture roles to a snapshot. Used by the process
// monitor so persisted process_metrics.csv carries process_role.
func (a *Aggregator) Classify(snap *process.Snapshot) {
	a.ownership.Classify(*snap)
}

// SetBaseline records the pre-workload process state as the baseline, without
// adding it to the measurement samples. Call before browsers/workers are
// created so the architecture RSS delta excludes setup memory.
func (a *Aggregator) SetBaseline(snap process.Snapshot) {
	a.ownership.Classify(snap)
	s := Aggregate(snap)
	a.mu.Lock()
	a.baseline = s
	a.haveBase = true
	a.mu.Unlock()
}

// Record aggregates a snapshot (roles must already be assigned via Classify).
func (a *Aggregator) Record(snap process.Snapshot) {
	s := Aggregate(snap)
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.haveBase {
		a.baseline = s
		a.haveBase = true
	}
	a.samples = append(a.samples, s)
}

// Baseline returns the first recorded sample (pre-workload when Record was
// called before tasks began).
func (a *Aggregator) Baseline() Sample {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.baseline
}

// Series returns the accumulated samples and whether a baseline exists.
func (a *Aggregator) Series() ([]Sample, Sample, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]Sample(nil), a.samples...), a.baseline, a.haveBase
}

// Delta computes the architecture RSS delta from the baseline (milestone
// section 5: architecture_rss_delta = benchmark_total_rss - baseline_rss).
func (a *Aggregator) Delta() uint64 {
	samples, base, ok := a.Series()
	if !ok || len(samples) == 0 {
		return 0
	}
	// Use the steady-state mean of benchmark RSS after the first sample to
	// avoid startup transients.
	var sum uint64
	n := 0
	for _, s := range samples[1:] {
		if s.BenchmarkRSS > 0 {
			sum += s.BenchmarkRSS
			n++
		}
	}
	if n == 0 {
		return 0
	}
	mean := sum / uint64(n)
	if mean > base.BenchmarkRSS {
		return mean - base.BenchmarkRSS
	}
	return 0
}
