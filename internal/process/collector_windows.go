//go:build windows

package process

import (
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// numCPU is the core count used to normalize per-process CPU% to the same
// denominator as the system-wide aggregate (percent of total cores).
var numCPU = runtime.NumCPU()

// Collector snapshots the Windows process table via Toolhelp32, augmenting
// each process with CPU (GetProcessTimes) and RSS (GetProcessMemoryInfo).
type Collector struct {
	last map[uint32]cpuSample
}

type cpuSample struct {
	at     time.Time
	user   uint64
	kern   uint64
	create windows.Filetime
}

// NewCollector returns a process-table collector.
func NewCollector() *Collector {
	return &Collector{last: make(map[uint32]cpuSample)}
}

// Sample walks the process table and returns a snapshot.
func (c *Collector) Sample() (Snapshot, error) {
	h, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return Snapshot{}, err
	}
	defer windows.CloseHandle(h)

	now := time.Now()
	entries := make([]Entry, 0, 512)
	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))

	for err := windows.Process32First(h, &pe); err == nil; err = windows.Process32Next(h, &pe) {
		e := Entry{
			PID:         pe.ProcessID,
			PPID:        pe.ParentProcessID,
			Name:        windows.UTF16ToString(pe.ExeFile[:]),
			ThreadCount: pe.Threads,
		}
		c.fillTimes(pe.ProcessID, now, &e)
		e.RSSBytes = workingSet(pe.ProcessID, &e)
		entries = append(entries, e)
	}
	return Snapshot{CapturedAt: now, Entries: entries}, nil
}

// fillTimes computes per-process CPU% from GetProcessTimes deltas, guarding
// against PID reuse via the process creation time.
func (c *Collector) fillTimes(pid uint32, now time.Time, e *Entry) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		e.AccessDenied = true
		return
	}
	defer windows.CloseHandle(h)

	var creation, exit, kern, user windows.Filetime
	if !getProcessTimes(h, &creation, &exit, &kern, &user) {
		e.AccessDenied = true
		return
	}
	userTicks := uint64(user.HighDateTime)<<32 | uint64(user.LowDateTime)
	kernTicks := uint64(kern.HighDateTime)<<32 | uint64(kern.LowDateTime)

	prev, ok := c.last[pid]
	if !ok || prev.create.HighDateTime != creation.HighDateTime || prev.create.LowDateTime != creation.LowDateTime {
		// First sighting, or the PID was reused by a different process.
		c.last[pid] = cpuSample{at: now, user: userTicks, kern: kernTicks, create: creation}
		return
	}
	userDelta := userTicks - prev.user
	kernDelta := kernTicks - prev.kern
	total := userDelta + kernDelta
	elapsed := now.Sub(prev.at).Seconds()
	c.last[pid] = cpuSample{at: now, user: userTicks, kern: kernTicks, create: creation}
	if elapsed <= 0 {
		return
	}
	// 100ns ticks → seconds of one core; normalize to percent of all cores so
	// this series agrees with the system-wide aggregate.
	pct := 100.0 * (float64(total) * 1e-7) / elapsed / float64(numCPU)
	if pct > 100 {
		pct = 100
	}
	if pct < 0 {
		pct = 0
	}
	e.CPUPercent = pct
}

// workingSet returns the process working set via GetProcessMemoryInfo.
func workingSet(pid uint32, e *Entry) uint64 {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		e.AccessDenied = true
		return 0
	}
	defer windows.CloseHandle(h)

	var pmc processMemoryCounters
	pmc.CB = uint32(unsafe.Sizeof(pmc))
	if getProcessMemoryInfo(h, &pmc) {
		return uint64(pmc.WorkingSetSize)
	}
	e.AccessDenied = true
	return 0
}
