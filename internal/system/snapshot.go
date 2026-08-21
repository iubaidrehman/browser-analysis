// Package system collects host-level resource snapshots for the telemetry
// phase: CPU, memory, swap, disk I/O, and network throughput sampled at a
// fixed interval during a benchmark run.
package system

import "time"

// Snapshot is one sample of host resource usage.
type Snapshot struct {
	// At is the sample capture time.
	At time.Time
	// CPUPercent is the system-wide CPU utilization since the previous sample.
	CPUPercent float64
	// RAMUsedBytes is the resident memory in use (excluding cache where
	// available).
	RAMUsedBytes uint64
	// RAMTotalBytes is the installed physical memory.
	RAMTotalBytes uint64
	// ProcessRSSBytes is the working set of the benchmark controller process.
	ProcessRSSBytes uint64
	// SwapUsedBytes is swap in use.
	SwapUsedBytes uint64
	// DiskReadBytes and DiskWriteBytes are cumulative since boot.
	DiskReadBytes  uint64
	DiskWriteBytes uint64
	// NetRXBytes and NetTXBytes are cumulative since boot.
	NetRXBytes uint64
	NetTXBytes uint64
	// ProcessCount and ThreadCount are host-wide counts.
	ProcessCount int
	ThreadCount  int
}
