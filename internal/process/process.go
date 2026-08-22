// Package process records the OS process tree during a benchmark run so the
// research can distinguish logical tasks from OS processes (spec section 13).
package process

import "time"

// Role classifies a process's part in the benchmark architecture.
type Role string

const (
	RoleUnknown   Role = "unknown"
	RoleController Role = "controller"
	RoleBrowser   Role = "browser"
	RoleRenderer  Role = "renderer"
	RoleUtility   Role = "utility"
	RoleGPU       Role = "gpu"
	RoleTarget    Role = "target"
	RoleAux       Role = "aux"
)

// Entry is one OS process at a snapshot instant.
type Entry struct {
	PID         uint32
	PPID        uint32
	Name        string
	CPUPercent  float64
	RSSBytes    uint64
	ThreadCount uint32
	// AccessDenied is true when the process could not be opened for CPU/RSS
	// sampling (e.g. elevated or system processes), distinguishing a failed
	// read from a genuine zero.
	AccessDenied bool
	// Role is the benchmark architecture role assigned by ownership detection.
	Role Role
}

// Snapshot is the full process table at a point in time.
type Snapshot struct {
	// CapturedAt is the wall-clock capture time.
	CapturedAt time.Time
	Entries    []Entry
}
