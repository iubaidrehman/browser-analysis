//go:build windows

package process

import (
	"unsafe"

	"golang.org/x/sys/windows"
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

var (
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	psapi    = windows.NewLazySystemDLL("psapi.dll")

	procGetProcessTimes  = kernel32.NewProc("GetProcessTimes")
	procGetProcessMemory = psapi.NewProc("GetProcessMemoryInfo")
)

func getProcessTimes(h windows.Handle, creation, exit, kern, user *windows.Filetime) bool {
	r1, _, _ := procGetProcessTimes.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(creation)),
		uintptr(unsafe.Pointer(exit)),
		uintptr(unsafe.Pointer(kern)),
		uintptr(unsafe.Pointer(user)),
	)
	return r1 != 0
}

func getProcessMemoryInfo(h windows.Handle, pmc *processMemoryCounters) bool {
	r1, _, _ := procGetProcessMemory.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(pmc)),
		uintptr(unsafe.Sizeof(*pmc)),
	)
	return r1 != 0
}
