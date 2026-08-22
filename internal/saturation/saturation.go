// Package saturation detects when a run crosses configured degradation
// thresholds relative to the scenario baseline (spec section 19).
package saturation

import (
	"bcrl/internal/results"
)

// Thresholds mirror spec section 19's initial defaults.
type Thresholds struct {
	// CPUMax is sustained CPU% that counts as saturation.
	CPUMax float64
	// P95Factor is how many times the baseline P95 counts as saturation.
	P95Factor float64
	// P99Factor is how many times the baseline P99 counts as saturation.
	P99Factor float64
	// FailureRateMax is the failure fraction that counts as saturation.
	FailureRateMax float64
}

// DefaultThresholds returns the spec's initial defaults.
func DefaultThresholds() Thresholds {
	return Thresholds{
		CPUMax:        90.0,
		P95Factor:     2.0,
		P99Factor:     3.0,
		FailureRateMax: 0.02,
	}
}

// Evaluation reports whether a run saturated and which thresholds crossed.
type Evaluation struct {
	Saturated    bool     `json:"saturated"`
	Violations   []string `json:"violations,omitempty"`
	BaselineP95  float64  `json:"baseline_p95"`
	BaselineP99  float64  `json:"baseline_p99"`
	BaselineCPU  float64  `json:"baseline_cpu"`
	BaselineRate float64  `json:"baseline_failure_rate"`
}

// Evaluate compares a run against the baseline (lowest concurrency) and the
// configured thresholds.
func Evaluate(run, baseline *results.Summary, t Thresholds) Evaluation {
	ev := Evaluation{
		BaselineP95:  baseline.Latency.P95,
		BaselineP99:  baseline.Latency.P99,
		BaselineCPU:  baseline.AvgCPU,
		BaselineRate: failureRate(baseline),
	}

	if t.CPUMax > 0 && run.AvgCPU > t.CPUMax {
		ev.Saturated = true
		ev.Violations = append(ev.Violations, "cpu")
	}
	if t.P95Factor > 0 && baseline.Latency.P95 > 0 &&
		run.Latency.P95 > t.P95Factor*baseline.Latency.P95 {
		ev.Saturated = true
		ev.Violations = append(ev.Violations, "p95")
	}
	if t.P99Factor > 0 && baseline.Latency.P99 > 0 &&
		run.Latency.P99 > t.P99Factor*baseline.Latency.P99 {
		ev.Saturated = true
		ev.Violations = append(ev.Violations, "p99")
	}
	if t.FailureRateMax > 0 && failureRate(run) > t.FailureRateMax {
		ev.Saturated = true
		ev.Violations = append(ev.Violations, "failure_rate")
	}
	return ev
}

// failureRate returns failed/total for a summary.
func failureRate(s *results.Summary) float64 {
	if s.TotalTasks == 0 {
		return 0
	}
	return float64(s.Failed) / float64(s.TotalTasks)
}
