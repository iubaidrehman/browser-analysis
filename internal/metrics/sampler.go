package metrics

import (
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"bcrl/internal/system"
)

// Sampler periodically captures host resource snapshots during a run. The
// samples are held in memory and written to system_metrics.csv by the
// results writer.
type Sampler struct {
	collector *system.Collector
	interval  time.Duration

	mu      sync.Mutex
	samples []system.Snapshot
	peakCPU float64
	avgCPU  float64
	peakRAM uint64
	avgRAM  uint64

	summaryMu      sync.Mutex
	summaryPeakCPU float64
	summaryAvgCPU  float64
	summaryPeakRAM uint64
	summaryAvgRAM  uint64
}

// NewSampler builds a sampler for the given interval.
func NewSampler(interval time.Duration) *Sampler {
	return &Sampler{
		collector: system.NewCollector(),
		interval:  interval,
	}
}

// Run samples until the context is cancelled. The returned func blocks until
// the sampler goroutine has fully stopped.
func (s *Sampler) Run(ctx context.Context) func() {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				snap, err := s.collector.Sample()
				if err != nil {
					continue
				}
				s.record(snap)
			}
		}
	}()
	return wg.Wait
}

func (s *Sampler) record(snap system.Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.samples = append(s.samples, snap)
	if snap.CPUPercent > s.peakCPU {
		s.peakCPU = snap.CPUPercent
	}
	if snap.RAMUsedBytes > s.peakRAM {
		s.peakRAM = snap.RAMUsedBytes
	}
	n := float64(len(s.samples))
	s.avgCPU = s.avgCPU*(n-1)/n + snap.CPUPercent/n
	s.avgRAM = uint64(float64(s.avgRAM)*(n-1)/n + float64(snap.RAMUsedBytes)/n)
}

// WriteCSV writes the samples to system_metrics.csv and records the
// aggregates for the summary builder.
func (s *Sampler) WriteCSV(dir string) error {
	s.mu.Lock()
	samples := append([]system.Snapshot(nil), s.samples...)
	peakCPU := s.peakCPU
	avgCPU := s.avgCPU
	peakRAM := s.peakRAM
	avgRAM := s.avgRAM
	s.mu.Unlock()

	path := filepath.Join(dir, "system_metrics.csv")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	header := []string{
		"timestamp_seconds", "cpu_percent", "ram_used_bytes", "ram_total_bytes",
		"process_rss_bytes", "swap_used_bytes", "disk_read_bytes", "disk_write_bytes",
		"net_rx_bytes", "net_tx_bytes", "process_count", "thread_count",
	}
	if err := w.Write(header); err != nil {
		return err
	}
	for _, s := range samples {
		row := []string{
			s.At.UTC().Format(time.RFC3339),
			strconv.FormatFloat(s.CPUPercent, 'f', 3, 64),
			strconv.FormatUint(s.RAMUsedBytes, 10),
			strconv.FormatUint(s.RAMTotalBytes, 10),
			strconv.FormatUint(s.ProcessRSSBytes, 10),
			strconv.FormatUint(s.SwapUsedBytes, 10),
			strconv.FormatUint(s.DiskReadBytes, 10),
			strconv.FormatUint(s.DiskWriteBytes, 10),
			strconv.FormatUint(s.NetRXBytes, 10),
			strconv.FormatUint(s.NetTXBytes, 10),
			strconv.Itoa(s.ProcessCount),
			strconv.Itoa(s.ThreadCount),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}

	s.summaryMu.Lock()
	s.summaryPeakCPU = peakCPU
	s.summaryAvgCPU = avgCPU
	s.summaryPeakRAM = peakRAM
	s.summaryAvgRAM = avgRAM
	s.summaryMu.Unlock()
	return nil
}

// Aggregates returns the peak/average CPU and RAM from the run.
func (s *Sampler) Aggregates() (peakCPU, avgCPU float64, peakRAM, avgRAM uint64) {
	s.summaryMu.Lock()
	defer s.summaryMu.Unlock()
	return s.summaryPeakCPU, s.summaryAvgCPU, s.summaryPeakRAM, s.summaryAvgRAM
}
