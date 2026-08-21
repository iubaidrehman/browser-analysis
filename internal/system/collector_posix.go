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

// Sample returns a host snapshot. The POSIX stub returns zeros.
func (c *Collector) Sample() (Snapshot, error) {
	now := time.Now()
	c.last = now
	return Snapshot{At: now}, nil
}
