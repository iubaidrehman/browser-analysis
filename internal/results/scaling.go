package results

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
)

// ScalingRow is one row of the scaling summary (milestone section 18).
type ScalingRow struct {
	Concurrency       int
	Throughput        float64
	P95               float64
	P99               float64
	CPU               float64
	RSSMeanMB         float64
	BrowserRSSMeanMB  float64
	ProcessMean       float64
	BrowserProcessMean float64
	Contexts          int
	FailureRate       float64
	Saturated         bool
}

// WriteScalingCSV writes scaling-summary.csv.
func WriteScalingCSV(dir string, rows []ScalingRow) error {
	path := filepath.Join(dir, "scaling-summary.csv")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write([]string{
		"concurrency", "throughput", "p95", "p99", "cpu",
		"rss_total_mean_mb", "browser_rss_mean_mb",
		"process_mean", "browser_process_mean", "context_count",
		"failure_rate", "saturated",
	}); err != nil {
		return err
	}
	for _, r := range rows {
		if err := w.Write([]string{
			strconv.Itoa(r.Concurrency),
			strconv.FormatFloat(r.Throughput, 'f', 2, 64),
			strconv.FormatFloat(r.P95, 'f', 4, 64),
			strconv.FormatFloat(r.P99, 'f', 4, 64),
			strconv.FormatFloat(r.CPU, 'f', 1, 64),
			strconv.FormatFloat(r.RSSMeanMB, 'f', 0, 64),
			strconv.FormatFloat(r.BrowserRSSMeanMB, 'f', 0, 64),
			strconv.FormatFloat(r.ProcessMean, 'f', 0, 64),
			strconv.FormatFloat(r.BrowserProcessMean, 'f', 0, 64),
			strconv.Itoa(r.Contexts),
			strconv.FormatFloat(r.FailureRate, 'f', 4, 64),
			strconv.FormatBool(r.Saturated),
		}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

// WriteResourceCSV writes resource-summary.csv (one row per run).
func WriteResourceCSV(dir string, rows []ResourceRow) error {
	path := filepath.Join(dir, "resource-summary.csv")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write([]string{
		"run_id", "scenario", "concurrency",
		"rss_total_mean_mb", "rss_total_p95_mb", "rss_total_peak_mb",
		"browser_rss_mean_mb", "browser_rss_p95_mb",
		"task_rss_mean_mb", "benchmark_cpu_mean",
		"process_mean", "browser_process_mean",
		"browsers", "contexts", "pages",
	}); err != nil {
		return err
	}
	for _, r := range rows {
		if err := w.Write([]string{
			r.RunID, r.Scenario, strconv.Itoa(r.Concurrency),
			strconv.FormatFloat(r.TotalRSSMeanMB, 'f', 0, 64),
			strconv.FormatFloat(r.TotalRSSP95MB, 'f', 0, 64),
			strconv.FormatFloat(r.TotalRSSPeakMB, 'f', 0, 64),
			strconv.FormatFloat(r.BrowserRSSMeanMB, 'f', 0, 64),
			strconv.FormatFloat(r.BrowserRSSP95MB, 'f', 0, 64),
			strconv.FormatFloat(r.TaskRSSMeanMB, 'f', 0, 64),
			strconv.FormatFloat(r.BenchmarkCPUMean, 'f', 1, 64),
			strconv.FormatFloat(r.ProcessMean, 'f', 0, 64),
			strconv.FormatFloat(r.BrowserProcessMean, 'f', 0, 64),
			strconv.Itoa(r.Browsers), strconv.Itoa(r.Contexts), strconv.Itoa(r.Pages),
		}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

// ResourceRow is one row of the resource summary (one per run).
type ResourceRow struct {
	RunID              string
	Scenario           string
	Concurrency        int
	TotalRSSMeanMB     float64
	TotalRSSP95MB      float64
	TotalRSSPeakMB     float64
	BrowserRSSMeanMB   float64
	BrowserRSSP95MB    float64
	TaskRSSMeanMB      float64
	BenchmarkCPUMean   float64
	ProcessMean        float64
	BrowserProcessMean float64
	Browsers           int
	Contexts           int
	Pages              int
}
