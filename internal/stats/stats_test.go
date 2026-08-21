package stats

import (
	"math"
	"testing"
)

func TestSummarize(t *testing.T) {
	vals := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	s := Summarize(vals)

	if s.Min != 1 || s.Max != 10 {
		t.Fatalf("min/max = %v/%v, want 1/10", s.Min, s.Max)
	}
	if math.Abs(s.Mean-5.5) > 1e-9 {
		t.Fatalf("mean = %v, want 5.5", s.Mean)
	}
	if s.Median != 5.5 {
		t.Fatalf("median = %v, want 5.5", s.Median)
	}
	if s.Count != 10 {
		t.Fatalf("count = %d, want 10", s.Count)
	}
}

func TestPercentile(t *testing.T) {
	vals := []float64{1, 2, 3, 4}
	// P50 of [1,2,3,4] is 2.5
	if got := Percentile(vals, 50); got != 2.5 {
		t.Fatalf("p50 = %v, want 2.5", got)
	}
}

func TestSummarizeEmpty(t *testing.T) {
	s := Summarize(nil)
	if s.Count != 0 {
		t.Fatalf("empty summary count = %d, want 0", s.Count)
	}
}
