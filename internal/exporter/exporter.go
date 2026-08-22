// Package exporter publishes benchmark run metrics via Prometheus text
// format. A single shared exporter serves /metrics for the process lifetime,
// and each completed run updates the gauges so a scrape reflects the latest
// results.
package exporter

import (
	"net/http"
	"strconv"
	"sync"

	"bcrl/internal/results"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	mu       sync.Mutex
	shared   *Exporter
	started  bool
	startErr error
)

// Shared returns the process-wide exporter, starting its /metrics server on
// the first call with a given address.
func Shared(addr string) *Exporter {
	mu.Lock()
	defer mu.Unlock()
	if shared == nil {
		shared = New()
	}
	if !started {
		mux := http.NewServeMux()
		mux.Handle("/metrics", shared.Handler())
		srv := &http.Server{Addr: addr, Handler: mux}
		go func() {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				mu.Lock()
				startErr = err
				mu.Unlock()
			}
		}()
		started = true
	}
	return shared
}

// Exporter owns the Prometheus registry and updates it per run.
type Exporter struct {
	registry *prometheus.Registry
	mu       sync.Mutex

	throughput  *prometheus.GaugeVec
	completed   *prometheus.GaugeVec
	failed      *prometheus.GaugeVec
	p95         *prometheus.GaugeVec
	p99         *prometheus.GaugeVec
	avgCPU      *prometheus.GaugeVec
	peakRAM     *prometheus.GaugeVec
	escalations *prometheus.GaugeVec
}

// New returns an exporter with the run metrics registered in a private
// registry (isolated from any other collectors in the process).
func New() *Exporter {
	e := &Exporter{registry: prometheus.NewRegistry()}
	e.throughput = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "bcrl_run_throughput",
		Help: "Completed workflows per measurement second.",
	}, []string{"scenario", "workflow", "concurrency"})
	e.completed = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "bcrl_run_completed",
		Help: "Completed tasks.",
	}, []string{"scenario", "workflow", "concurrency"})
	e.failed = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "bcrl_run_failed",
		Help: "Failed tasks.",
	}, []string{"scenario", "workflow", "concurrency"})
	e.p95 = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "bcrl_run_p95_seconds",
		Help: "P95 workflow latency.",
	}, []string{"scenario", "workflow", "concurrency"})
	e.p99 = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "bcrl_run_p99_seconds",
		Help: "P99 workflow latency.",
	}, []string{"scenario", "workflow", "concurrency"})
	e.avgCPU = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "bcrl_run_avg_cpu_percent",
		Help: "Average host CPU percent during the run.",
	}, []string{"scenario", "workflow", "concurrency"})
	e.peakRAM = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "bcrl_run_peak_ram_bytes",
		Help: "Peak host RAM bytes during the run.",
	}, []string{"scenario", "workflow", "concurrency"})
	e.escalations = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "bcrl_run_escalations",
		Help: "Browser escalations (hybrid scenario).",
	}, []string{"scenario", "workflow", "concurrency"})

	e.registry.MustRegister(
		e.throughput, e.completed, e.failed, e.p95, e.p99,
		e.avgCPU, e.peakRAM, e.escalations,
	)
	return e
}

// Record updates the gauges with a completed run's summary.
func (e *Exporter) Record(s *results.Summary) {
	e.mu.Lock()
	defer e.mu.Unlock()

	labels := prometheus.Labels{
		"scenario":    s.Scenario,
		"workflow":    s.Workflow,
		"concurrency": strconv.Itoa(s.Concurrency),
	}
	e.throughput.With(labels).Set(s.Throughput)
	e.completed.With(labels).Set(float64(s.Completed))
	e.failed.With(labels).Set(float64(s.Failed))
	e.p95.With(labels).Set(s.Latency.P95)
	e.p99.With(labels).Set(s.Latency.P99)
	e.avgCPU.With(labels).Set(s.AvgCPU)
	e.peakRAM.With(labels).Set(float64(s.PeakRAMBytes))
	e.escalations.With(labels).Set(float64(s.Escalations))
}

// Handler returns the Prometheus scrape handler for this exporter's registry.
func (e *Exporter) Handler() http.Handler {
	return promhttp.HandlerFor(e.registry, promhttp.HandlerOpts{})
}
