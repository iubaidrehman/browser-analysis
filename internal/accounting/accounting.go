// Package accounting aggregates per-role process metrics into the resource
// model: benchmark total RSS, browser tree RSS, controller RSS, target RSS,
// and per-role process counts (milestone: aggregate resource accounting).
package accounting

import (
	"sort"

	"bcrl/internal/process"
)

// Sample is one snapshot's aggregated resource state.
type Sample struct {
	TimestampUnix int64

	// RSS by role (bytes).
	ControllerRSS uint64
	BrowserRSS    uint64
	TargetRSS     uint64
	AuxRSS        uint64
	BenchmarkRSS  uint64 // controller + browser + target + aux

	// CPU by role (percent, one-core normalized by the collector).
	ControllerCPU float64
	BrowserCPU    float64
	TargetCPU     float64
	BenchmarkCPU  float64

	// Process counts.
	TotalProcesses    int
	BrowserProcesses  int
	RendererProcesses int
	UtilityProcesses  int
	GPUProcesses      int
	ControllerProcesses int
	TargetProcesses   int
}

// Aggregate reduces a process snapshot into a Sample. RSS is summed per role
// without double-counting: each process appears once, under exactly one role.
func Aggregate(snap process.Snapshot) Sample {
	s := Sample{TimestampUnix: snap.CapturedAt.Unix()}
	for _, e := range snap.Entries {
		switch e.Role {
		case process.RoleController:
			s.ControllerRSS += e.RSSBytes
			s.ControllerCPU += e.CPUPercent
			s.ControllerProcesses++
		case process.RoleBrowser:
			s.BrowserRSS += e.RSSBytes
			s.BrowserCPU += e.CPUPercent
			s.BrowserProcesses++
		case process.RoleRenderer:
			s.BrowserRSS += e.RSSBytes
			s.BrowserCPU += e.CPUPercent
			s.RendererProcesses++
			s.BrowserProcesses++
		case process.RoleUtility:
			s.BrowserRSS += e.RSSBytes
			s.BrowserCPU += e.CPUPercent
			s.UtilityProcesses++
			s.BrowserProcesses++
		case process.RoleGPU:
			s.BrowserRSS += e.RSSBytes
			s.BrowserCPU += e.CPUPercent
			s.GPUProcesses++
			s.BrowserProcesses++
		case process.RoleTarget:
			s.TargetRSS += e.RSSBytes
			s.TargetCPU += e.CPUPercent
			s.TargetProcesses++
		default:
			s.AuxRSS += e.RSSBytes
		}
	}
	s.BenchmarkRSS = s.ControllerRSS + s.BrowserRSS + s.TargetRSS + s.AuxRSS
	s.BenchmarkCPU = s.ControllerCPU + s.BrowserCPU + s.TargetCPU
	s.TotalProcesses = len(snap.Entries)
	return s
}

// Series holds the per-snapshot aggregated samples of a run.
type Series struct {
	Baseline Sample
	Samples  []Sample
}

// Percentiles computes p50/p95/peak over a slice of uint64 values.
type Uint64Stats struct {
	Mean  uint64
	P50   uint64
	P95   uint64
	Peak  uint64
}

// Stats computes summary stats for an RSS series.
func Stats(vals []uint64) Uint64Stats {
	if len(vals) == 0 {
		return Uint64Stats{}
	}
	sorted := append([]uint64(nil), vals...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	var sum uint64
	for _, v := range vals {
		sum += v
	}
	return Uint64Stats{
		Mean: sum / uint64(len(vals)),
		P50:  percentile(sorted, 50),
		P95:  percentile(sorted, 95),
		Peak: sorted[len(sorted)-1],
	}
}

func percentile(sorted []uint64, p int) uint64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}
	// Nearest-rank: index = ceil(p/100 * n) - 1.
	idx := (p*n + 99) / 100
	if idx >= 1 {
		idx--
	}
	if idx >= n {
		idx = n - 1
	}
	return sorted[idx]
}

// Float64Stats computes summary stats for a float series.
type Float64Stats struct {
	Mean float64
	P50  float64
	P95  float64
	Peak float64
}

// FStats computes summary stats for a float series.
func FStats(vals []float64) Float64Stats {
	if len(vals) == 0 {
		return Float64Stats{}
	}
	sorted := append([]float64(nil), vals...)
	sort.Float64s(sorted)
	var sum float64
	for _, v := range vals {
		sum += v
	}
	nearest := func(p int) float64 {
		idx := (p*len(sorted) + 99) / 100
		if idx >= 1 {
			idx--
		}
		if idx >= len(sorted) {
			idx = len(sorted) - 1
		}
		return sorted[idx]
	}
	return Float64Stats{
		Mean: sum / float64(len(vals)),
		P50:  nearest(50),
		P95:  nearest(95),
		Peak: sorted[len(sorted)-1],
	}
}
