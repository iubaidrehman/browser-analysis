package saturation

import (
	"testing"

	"bcrl/internal/results"
)

func TestEvaluateNoSaturation(t *testing.T) {
	baseline := &results.Summary{TotalTasks: 100, Failed: 0}
	baseline.Latency.P95 = 0.1
	baseline.Latency.P99 = 0.2

	run := &results.Summary{TotalTasks: 100, Failed: 1}
	run.Latency.P95 = 0.15
	run.Latency.P99 = 0.25
	run.AvgCPU = 50

	ev := Evaluate(run, baseline, DefaultThresholds())
	if ev.Saturated {
		t.Fatalf("expected no saturation, got %v", ev.Violations)
	}
}

func TestEvaluateSaturationLatency(t *testing.T) {
	baseline := &results.Summary{TotalTasks: 100, Failed: 0}
	baseline.Latency.P95 = 0.1
	baseline.Latency.P99 = 0.2

	run := &results.Summary{TotalTasks: 100, Failed: 0}
	run.Latency.P95 = 0.25 // > 2x baseline
	run.Latency.P99 = 0.5

	ev := Evaluate(run, baseline, DefaultThresholds())
	if !ev.Saturated {
		t.Fatal("expected saturation from p95")
	}
	found := false
	for _, v := range ev.Violations {
		if v == "p95" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected p95 violation, got %v", ev.Violations)
	}
}

func TestEvaluateSaturationFailureRate(t *testing.T) {
	baseline := &results.Summary{TotalTasks: 100, Failed: 0}
	baseline.Latency.P95 = 0.1
	baseline.Latency.P99 = 0.2

	run := &results.Summary{TotalTasks: 100, Failed: 10} // 10% > 2%
	run.Latency.P95 = 0.1
	run.Latency.P99 = 0.2

	ev := Evaluate(run, baseline, DefaultThresholds())
	if !ev.Saturated {
		t.Fatal("expected saturation from failure rate")
	}
}
