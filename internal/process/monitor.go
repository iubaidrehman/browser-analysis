package process

import (
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// Monitor periodically snapshots the process table during a run and writes
// the accumulated snapshots to process_metrics.csv.
type Monitor struct {
	collector *Collector
	interval  time.Duration

	mu        sync.Mutex
	snapshots []Snapshot

	// Sink, when set, receives every snapshot (including the seed) so an
	// aggregator can consume it without coupling to the monitor.
	Sink func(Snapshot)
	// Classify, when set, assigns roles to each snapshot before it is stored
	// and sent to the sink (so process_metrics.csv carries process_role).
	Classify func(*Snapshot)
}

// NewMonitor builds a monitor with the given snapshot interval.
func NewMonitor(interval time.Duration) *Monitor {
	return &Monitor{
		collector: NewCollector(),
		interval:  interval,
	}
}

// Run samples until the context is cancelled. Returns a wait func.
func (m *Monitor) Run(ctx context.Context) func() {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Seed once so the first tick carries real CPU deltas.
		if snap, err := m.collector.Sample(); err == nil {
			if m.Classify != nil {
				m.Classify(&snap)
			}
			m.mu.Lock()
			m.snapshots = append(m.snapshots, snap)
			m.mu.Unlock()
			if m.Sink != nil {
				m.Sink(snap)
			}
		}
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				snap, err := m.collector.Sample()
				if err != nil {
					continue
				}
				if m.Classify != nil {
					m.Classify(&snap)
				}
				m.mu.Lock()
				m.snapshots = append(m.snapshots, snap)
				m.mu.Unlock()
				if m.Sink != nil {
					m.Sink(snap)
				}
			}
		}
	}()
	return wg.Wait
}

// WriteCSV writes process_metrics.csv with one row per process per snapshot.
func (m *Monitor) WriteCSV(dir string) error {
	m.mu.Lock()
	snaps := append([]Snapshot(nil), m.snapshots...)
	m.mu.Unlock()

	path := filepath.Join(dir, "process_metrics.csv")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write([]string{
		"timestamp", "pid", "ppid", "name", "process_role", "cpu_percent",
		"rss_bytes", "thread_count", "access_denied",
	}); err != nil {
		return err
	}
	for _, snap := range snaps {
		ts := snap.CapturedAt.UTC().Format(time.RFC3339)
		for _, e := range snap.Entries {
			row := []string{
				ts,
				strconv.FormatUint(uint64(e.PID), 10),
				strconv.FormatUint(uint64(e.PPID), 10),
				e.Name,
				string(e.Role),
				strconv.FormatFloat(e.CPUPercent, 'f', 3, 64),
				strconv.FormatUint(e.RSSBytes, 10),
				strconv.FormatUint(uint64(e.ThreadCount), 10),
				strconv.FormatBool(e.AccessDenied),
			}
			if err := w.Write(row); err != nil {
				return err
			}
		}
	}
	w.Flush()
	return w.Error()
}
