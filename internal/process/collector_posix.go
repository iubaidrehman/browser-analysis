//go:build !windows

package process

import "time"

// Collector is a stub on non-Windows platforms.
type Collector struct {
	last time.Time
}

// NewCollector returns a collector.
func NewCollector() *Collector { return &Collector{} }

// Sample returns an empty snapshot on POSIX.
func (c *Collector) Sample() (Snapshot, error) {
	c.last = time.Now()
	return Snapshot{}, nil
}
