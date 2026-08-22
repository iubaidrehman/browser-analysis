//go:build windows

package system

import (
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32           = windows.NewLazySystemDLL("kernel32.dll")
	psapi              = windows.NewLazySystemDLL("psapi.dll")
	procGetSystemTimes = kernel32.NewProc("GetSystemTimes")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
	procGetProcessMemoryInfo = psapi.NewProc("GetProcessMemoryInfo")
)

type processMemoryCounters struct {
	CB                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
}

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

// Collector samples host metrics via the Windows API.
type Collector struct {
	lastCPU time.Time
	lastIdle, lastKernel, lastUser uint64
}

// ProcessRSS returns the working set (RSS) of the current process in bytes.
// Used to measure per-task memory deltas (the benchmark process's children,
// including Chromium, appear in its working set).
func ProcessRSS() uint64 {
	var pmc processMemoryCounters
	pmc.CB = uint32(unsafe.Sizeof(pmc))
	r1, _, _ := procGetProcessMemoryInfo.Call(
		uintptr(windows.CurrentProcess()),
		uintptr(unsafe.Pointer(&pmc)),
		uintptr(unsafe.Sizeof(pmc)),
	)
	if r1 == 0 {
		return 0
	}
	return uint64(pmc.WorkingSetSize)
}

// TreeRSS returns the total working set of the process rooted at rootPID and
// all of its descendants. Used to measure per-task memory including spawned
// Chromium trees, which are separate processes not counted in the parent's
// working set.
func TreeRSS(rootPID uint32) uint64 {
	h, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(h)

	type procInfo struct {
		ppid uint32
		rss  uint64
	}
	procs := make(map[uint32]procInfo, 512)
	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	for err := windows.Process32First(h, &pe); err == nil; err = windows.Process32Next(h, &pe) {
		procs[pe.ProcessID] = procInfo{ppid: pe.ParentProcessID, rss: processRSS(pe.ProcessID)}
	}

	// Sum the root and every process that descends from it.
	total := procs[rootPID].rss
	for _, info := range procs {
		for cur := info.ppid; cur != 0; {
			if cur == rootPID {
				total += info.rss
				break
			}
			parent, ok := procs[cur]
			if !ok {
				break
			}
			cur = parent.ppid
		}
	}
	return total
}

func processRSS(pid uint32) uint64 {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(h)
	var pmc processMemoryCounters
	pmc.CB = uint32(unsafe.Sizeof(pmc))
	r1, _, _ := procGetProcessMemoryInfo.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(&pmc)),
		uintptr(unsafe.Sizeof(pmc)),
	)
	if r1 == 0 {
		return 0
	}
	return uint64(pmc.WorkingSetSize)
}

// NewCollector returns a collector ready to sample.
func NewCollector() *Collector {
	return &Collector{}
}

// Sample returns a host snapshot. CPU percent is computed from the delta of
// system times between consecutive samples; the first call returns 0.
func (c *Collector) Sample() (Snapshot, error) {
	now := time.Now()

	// CPU: GetSystemTimes provides idle/kernel/user 100ns ticks.
	var idle, kernel, user windows.Filetime
	r1, _, err := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idle)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)),
	)
	if r1 == 0 {
		return Snapshot{}, err
	}
	idleTicks := uint64(idle.HighDateTime)<<32 | uint64(idle.LowDateTime)
	kernelTicks := uint64(kernel.HighDateTime)<<32 | uint64(kernel.LowDateTime)
	userTicks := uint64(user.HighDateTime)<<32 | uint64(user.LowDateTime)

	var cpu float64
	if !c.lastCPU.IsZero() {
		idleDelta := idleTicks - c.lastIdle
		kernelDelta := kernelTicks - c.lastKernel
		userDelta := userTicks - c.lastUser
		// On Windows the kernel time INCLUDES idle time, so busy time is
		// (kernel - idle) + user, and the denominator is kernel + user.
		busy := (kernelDelta - idleDelta) + userDelta
		total := kernelDelta + userDelta
		if total > 0 {
			cpu = 100.0 * float64(busy) / float64(total)
		}
	}
	c.lastCPU = now
	c.lastIdle, c.lastKernel, c.lastUser = idleTicks, kernelTicks, userTicks

	// Memory: GlobalMemoryStatusEx.
	var mem memoryStatusEx
	mem.Length = uint32(unsafe.Sizeof(mem))
	r1, _, err = procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&mem)))
	if r1 == 0 {
		return Snapshot{}, err
	}

	// Process RSS: working set of the current (benchmark controller) process.
	var pmc processMemoryCounters
	pmc.CB = uint32(unsafe.Sizeof(pmc))
	r1, _, err = procGetProcessMemoryInfo.Call(
		uintptr(windows.CurrentProcess()),
		uintptr(unsafe.Pointer(&pmc)),
		uintptr(unsafe.Sizeof(pmc)),
	)
	var rss uint64
	if r1 != 0 {
		rss = uint64(pmc.WorkingSetSize)
	}

	procCount := countProcesses()
	threadCount := countThreads()

	return Snapshot{
		At:              now,
		CPUPercent:      cpu,
		RAMUsedBytes:    mem.TotalPhys - mem.AvailPhys,
		RAMTotalBytes:   mem.TotalPhys,
		ProcessRSSBytes: rss,
		SwapUsedBytes:   mem.TotalPageFile - mem.AvailPageFile,
		ProcessCount:    procCount,
		ThreadCount:     threadCount,
	}, nil
}

func countProcesses() int {
	// NtQuerySystemInformation enumeration is heavy; use a Toolhelp32
	// snapshot for process and thread counts.
	h, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(h)

	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	n := 0
	if err := windows.Process32First(h, &pe); err == nil {
		n++
		for windows.Process32Next(h, &pe) == nil {
			n++
		}
	}
	return n
}

func countThreads() int {
	h, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(h)

	var te windows.ThreadEntry32
	te.Size = uint32(unsafe.Sizeof(te))
	n := 0
	if err := windows.Thread32First(h, &te); err == nil {
		n++
		for windows.Thread32Next(h, &te) == nil {
			n++
		}
	}
	return n
}
