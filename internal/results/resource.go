package results

import (
	"bcrl/internal/accounting"
)

// ResourceSummary carries the aggregate resource accounting for one run
// (milestone sections 6-8, 13-14). All RSS values are in bytes unless stated.
type ResourceSummary struct {
	// Baseline memory (milestone section 5).
	BaselineTotalRSS uint64 `json:"baseline_total_rss"`

	// Architecture RSS delta (primary scalability metric).
	ArchitectureRSSDelta uint64 `json:"architecture_rss_delta"`

	// Total benchmark RSS series.
	TotalRSS accounting.Uint64Stats `json:"rss_total"`
	// Browser tree RSS series.
	BrowserRSS accounting.Uint64Stats `json:"browser_rss"`
	// Controller RSS series.
	ControllerRSS accounting.Uint64Stats `json:"controller_rss"`
	// Target RSS series.
	TargetRSS accounting.Uint64Stats `json:"target_rss"`

	// Per-task RSS delta series (existing metric, kept).
	TaskRSS accounting.Uint64Stats `json:"task_rss_delta"`

	// CPU splits (percent of one core via the process collector).
	BenchmarkCPU accounting.Float64Stats `json:"benchmark_cpu"`
	BrowserCPU   accounting.Float64Stats `json:"browser_cpu"`
	ControllerCPU accounting.Float64Stats `json:"controller_cpu"`
	TargetCPU    accounting.Float64Stats `json:"target_cpu"`

	// Process counts.
	TotalProcesses    accounting.Uint64Stats `json:"process_total"`
	BrowserProcesses  accounting.Uint64Stats `json:"process_browser"`
	RendererProcesses accounting.Uint64Stats `json:"process_renderer"`
	UtilityProcesses  accounting.Uint64Stats `json:"process_utility"`
	GPUProcesses      accounting.Uint64Stats `json:"process_gpu"`
	ControllerProcesses accounting.Uint64Stats `json:"process_controller"`
	TargetProcesses   accounting.Uint64Stats `json:"process_target"`

	// Lifecycle counts measured from events (milestone section 9).
	Browsers  int `json:"browsers"`
	Contexts  int `json:"contexts"`
	Pages     int `json:"pages"`
	Workers   int `json:"active_workers"`

	// Derived scaling metrics (milestone sections 13-14).
	MemoryPerLogicalTask uint64  `json:"memory_delta_per_logical_task"`
	BrowserMemoryPerContext uint64 `json:"browser_memory_delta_per_context"`
	MemoryPerActiveWorker uint64 `json:"memory_delta_per_active_worker"`
	ThroughputPerCPU     float64 `json:"throughput_per_cpu_percent"`
	ThroughputPerGBRSS   float64 `json:"throughput_per_gb_rss"`
}
