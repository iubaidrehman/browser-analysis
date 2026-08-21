// Package stats computes latency distributions and run summaries.
package stats

import (
	"math"
	"sort"
)

// Percentile returns the p-th percentile (0-100) of sorted-ish values.
// It uses linear interpolation between closest ranks.
func Percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}
	rank := p / 100 * float64(n-1)
	lo := int(math.Floor(rank))
	hi := int(math.Ceil(rank))
	if lo == hi {
		return sorted[lo]
	}
	frac := rank - float64(lo)
	return sorted[lo] + frac*(sorted[hi]-sorted[lo])
}

// Summary is a full latency distribution summary.
type Summary struct {
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Mean   float64 `json:"mean"`
	Median float64 `json:"median"`
	P90    float64 `json:"p90"`
	P95    float64 `json:"p95"`
	P99    float64 `json:"p99"`
	StdDev float64 `json:"stddev"`
	Count  int     `json:"count"`
}

// Summarize computes a latency Summary from raw duration values (in seconds).
func Summarize(values []float64) Summary {
	if len(values) == 0 {
		return Summary{}
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))

	var sq float64
	for _, v := range values {
		d := v - mean
		sq += d * d
	}
	stddev := math.Sqrt(sq / float64(len(values)))

	return Summary{
		Min:    sorted[0],
		Max:    sorted[len(sorted)-1],
		Mean:   mean,
		Median: Percentile(sorted, 50),
		P90:    Percentile(sorted, 90),
		P95:    Percentile(sorted, 95),
		P99:    Percentile(sorted, 99),
		StdDev: stddev,
		Count:  len(sorted),
	}
}
