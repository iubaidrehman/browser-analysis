package accounting

import (
	"testing"
	"time"

	"bcrl/internal/process"
)

func TestAggregateNoDoubleCount(t *testing.T) {
	snap := process.Snapshot{
		CapturedAt: time.Now(),
		Entries: []process.Entry{
			{PID: 1, PPID: 0, Name: "bench.exe", RSSBytes: 100, Role: process.RoleController, CPUPercent: 5},
			{PID: 2, PPID: 1, Name: "chrome.exe", RSSBytes: 200, Role: process.RoleBrowser, CPUPercent: 10},
			{PID: 3, PPID: 2, Name: "renderer", RSSBytes: 300, Role: process.RoleRenderer, CPUPercent: 20},
			{PID: 4, PPID: 2, Name: "utility", RSSBytes: 400, Role: process.RoleUtility, CPUPercent: 30},
			{PID: 5, PPID: 2, Name: "gpu", RSSBytes: 500, Role: process.RoleGPU, CPUPercent: 40},
			{PID: 6, PPID: 0, Name: "backend.exe", RSSBytes: 600, Role: process.RoleTarget, CPUPercent: 2},
			{PID: 7, PPID: 1, Name: "other.exe", RSSBytes: 700, Role: process.RoleAux, CPUPercent: 0},
		},
	}

	s := Aggregate(snap)
	// Browser tree = browser + renderer + utility + gpu.
	wantBrowser := uint64(200 + 300 + 400 + 500)
	if s.BrowserRSS != wantBrowser {
		t.Fatalf("browser rss = %d, want %d", s.BrowserRSS, wantBrowser)
	}
	// Benchmark total = all benchmark-owned roles (excludes unknown).
	wantTotal := uint64(100 + 200 + 300 + 400 + 500 + 600 + 700)
	if s.BenchmarkRSS != wantTotal {
		t.Fatalf("benchmark rss = %d, want %d", s.BenchmarkRSS, wantTotal)
	}
	// No double counting: sum of role RSS equals benchmark total.
	if s.ControllerRSS+s.BrowserRSS+s.TargetRSS+s.AuxRSS != s.BenchmarkRSS {
		t.Fatal("role RSS sum does not equal benchmark total (double counting)")
	}
	// CPU aggregation.
	if s.BrowserCPU != 10+20+30+40 {
		t.Fatalf("browser cpu = %v, want 100", s.BrowserCPU)
	}
	// Process counts.
	if s.BrowserProcesses != 4 || s.RendererProcesses != 1 || s.UtilityProcesses != 1 || s.GPUProcesses != 1 {
		t.Fatalf("browser process counts wrong: %+v", s)
	}
}

func TestOwnershipClassification(t *testing.T) {
	o := &process.Ownership{ControllerPID: 1, TargetPID: 9}
	snap := process.Snapshot{
		CapturedAt: time.Now(),
		Entries: []process.Entry{
			{PID: 1, PPID: 0, Name: "bench.exe"},
			{PID: 2, PPID: 1, Name: "chrome-headless-shell.exe"},
			{PID: 3, PPID: 2, Name: "chrome-headless-shell.exe"},
			{PID: 4, PPID: 2, Name: "utility"},
			{PID: 5, PPID: 2, Name: "gpu_process"},
			{PID: 6, PPID: 1, Name: "node.exe"},
			{PID: 9, PPID: 0, Name: "backend.exe"},
			{PID: 10, PPID: 0, Name: "system.exe"},
		},
	}
	o.Classify(snap)

	roles := map[uint32]process.Role{}
	for _, e := range snap.Entries {
		roles[e.PID] = e.Role
	}
	if roles[1] != process.RoleController {
		t.Fatalf("pid 1 role = %v, want controller", roles[1])
	}
	// Topmost chromium child of the controller is the browser parent.
	if roles[2] != process.RoleBrowser {
		t.Fatalf("pid 2 role = %v, want browser", roles[2])
	}
	// Child of the browser with a chromium name is a renderer.
	if roles[3] != process.RoleRenderer {
		t.Fatalf("pid 3 role = %v, want renderer", roles[3])
	}
	if roles[4] != process.RoleUtility {
		t.Fatalf("pid 4 role = %v, want utility", roles[4])
	}
	if roles[5] != process.RoleGPU {
		t.Fatalf("pid 5 role = %v, want gpu", roles[5])
	}
	// Non-chromium child of the controller is auxiliary.
	if roles[6] != process.RoleAux {
		t.Fatalf("pid 6 role = %v, want aux", roles[6])
	}
	if roles[9] != process.RoleTarget {
		t.Fatalf("pid 9 role = %v, want target", roles[9])
	}
	// Unrelated system process stays unknown.
	if roles[10] != process.RoleUnknown {
		t.Fatalf("pid 10 role = %v, want unknown", roles[10])
	}
}

func TestAggregatorBaselineAndDelta(t *testing.T) {
	agg := NewAggregator(1, 0)
	base := process.Snapshot{CapturedAt: time.Now(), Entries: []process.Entry{
		{PID: 1, PPID: 0, Name: "bench.exe", RSSBytes: 100, Role: process.RoleController},
	}}
	agg.Record(base)
	loaded := process.Snapshot{CapturedAt: time.Now(), Entries: []process.Entry{
		{PID: 1, PPID: 0, Name: "bench.exe", RSSBytes: 100, Role: process.RoleController},
		{PID: 2, PPID: 1, Name: "chrome.exe", RSSBytes: 500, Role: process.RoleBrowser},
		{PID: 3, PPID: 2, Name: "renderer", RSSBytes: 400, Role: process.RoleRenderer},
	}}
	agg.Record(loaded)
	agg.Record(loaded)

	if agg.Baseline().BenchmarkRSS != 100 {
		t.Fatalf("baseline = %d, want 100", agg.Baseline().BenchmarkRSS)
	}
	// Delta = mean loaded total (1000) - baseline (100) = 900.
	if d := agg.Delta(); d != 900 {
		t.Fatalf("delta = %d, want 900", d)
	}
}

func TestStatsPercentiles(t *testing.T) {
	vals := []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	s := Stats(vals)
	if s.Peak != 10 || s.Mean != 5 {
		t.Fatalf("peak=%d mean=%d", s.Peak, s.Mean)
	}
	if s.P50 != 5 || s.P95 != 10 {
		t.Fatalf("p50=%d p95=%d", s.P50, s.P95)
	}
	// Single sample: percentiles equal the value.
	one := Stats([]uint64{42})
	if one.P95 != 42 || one.P50 != 42 {
		t.Fatalf("single sample stats wrong: %+v", one)
	}
}
