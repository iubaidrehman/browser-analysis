package results

import (
	"bcrl/internal/accounting"
	"bcrl/internal/metrics"
)

// BuildResourceSummary assembles the ResourceSummary for a run from the
// aggregator's series and the recorder's lifecycle/task data.
func BuildResourceSummary(agg *accounting.Aggregator, rec *metrics.Recorder, concurrency int, throughput float64) ResourceSummary {
	samples, base, ok := agg.Series()
	if !ok {
		return ResourceSummary{}
	}

	rs := ResourceSummary{
		BaselineTotalRSS:    base.BenchmarkRSS,
		ArchitectureRSSDelta: agg.Delta(),
		TaskRSS:             uint64Stats(rec.RSSDeltas()),
	}

	var (
		totalRSS, browserRSS, controllerRSS, targetRSS     []uint64
		totalCPU, browserCPU, controllerCPU, targetCPU     []float64
		totalProc, browserProc, rendererProc, utilityProc  []uint64
		gpuProc, controllerProc, targetProc                []uint64
	)
	for _, s := range samples {
		totalRSS = append(totalRSS, s.BenchmarkRSS)
		browserRSS = append(browserRSS, s.BrowserRSS)
		controllerRSS = append(controllerRSS, s.ControllerRSS)
		targetRSS = append(targetRSS, s.TargetRSS)
		totalCPU = append(totalCPU, s.BenchmarkCPU)
		browserCPU = append(browserCPU, s.BrowserCPU)
		controllerCPU = append(controllerCPU, s.ControllerCPU)
		targetCPU = append(targetCPU, s.TargetCPU)
		totalProc = append(totalProc, uint64(s.TotalProcesses))
		browserProc = append(browserProc, uint64(s.BrowserProcesses))
		rendererProc = append(rendererProc, uint64(s.RendererProcesses))
		utilityProc = append(utilityProc, uint64(s.UtilityProcesses))
		gpuProc = append(gpuProc, uint64(s.GPUProcesses))
		controllerProc = append(controllerProc, uint64(s.ControllerProcesses))
		targetProc = append(targetProc, uint64(s.TargetProcesses))
	}

	rs.TotalRSS = accounting.Stats(totalRSS)
	rs.BrowserRSS = accounting.Stats(browserRSS)
	rs.ControllerRSS = accounting.Stats(controllerRSS)
	rs.TargetRSS = accounting.Stats(targetRSS)
	rs.BenchmarkCPU = accounting.FStats(totalCPU)
	rs.BrowserCPU = accounting.FStats(browserCPU)
	rs.ControllerCPU = accounting.FStats(controllerCPU)
	rs.TargetCPU = accounting.FStats(targetCPU)
	rs.TotalProcesses = accounting.Stats(totalProc)
	rs.BrowserProcesses = accounting.Stats(browserProc)
	rs.RendererProcesses = accounting.Stats(rendererProc)
	rs.UtilityProcesses = accounting.Stats(utilityProc)
	rs.GPUProcesses = accounting.Stats(gpuProc)
	rs.ControllerProcesses = accounting.Stats(controllerProc)
	rs.TargetProcesses = accounting.Stats(targetProc)

	// Lifecycle counts measured from events.
	rs.Browsers, rs.Contexts, rs.Pages = rec.LifecycleCounts()
	rs.Workers = concurrency

	// Derived scaling metrics.
	if concurrency > 0 {
		rs.MemoryPerLogicalTask = rs.ArchitectureRSSDelta / uint64(concurrency)
	}
	if rs.Contexts > 0 {
		rs.BrowserMemoryPerContext = rs.BrowserRSS.Mean / uint64(rs.Contexts)
	}
	if concurrency > 0 {
		rs.MemoryPerActiveWorker = rs.ArchitectureRSSDelta / uint64(concurrency)
	}
	if rs.BenchmarkCPU.Mean > 0 {
		rs.ThroughputPerCPU = throughput / rs.BenchmarkCPU.Mean
	}
	if rs.TotalRSS.Mean > 0 {
		rs.ThroughputPerGBRSS = throughput / (float64(rs.TotalRSS.Mean) / (1024 * 1024 * 1024))
	}
	return rs
}

func uint64Stats(vals []float64) accounting.Uint64Stats {
	u := make([]uint64, len(vals))
	for i, v := range vals {
		u[i] = uint64(v)
	}
	return accounting.Stats(u)
}
