//go:build !windows

package system

import "time"

// Collector samples host metrics. The POSIX implementation uses the gopsutil
// package; this stub keeps the build green on non-Windows until it lands.
type Collector struct {
	last time.Time
}

// NewCollector returns a collector ready to sample.
func NewCollector() *Collector { return &Collector{} }

// ProcessRSS is unimplemented on POSIX.
func ProcessRSS() uint64 { return 0 }

// TreeRSS is unimplemented on POSIX.
func TreeRSS(rootPID uint32) uint64 { return 0 }

// Sample returns a host snapshot. The POSIX stub returns zeros.
func (c *Collector) Sample() (Snapshot, error) {
	now := time.Now()
	c.last = now
	return Snapshot{At: now}, nil
}
