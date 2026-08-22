package process

import "strings"

// Ownership classifies processes into benchmark architecture roles.
type Ownership struct {
	// ControllerPID is the benchmark controller process id.
	ControllerPID uint32
	// TargetPID is the synthetic backend process id (0 if not separately
	// spawned, e.g. when running under Docker or go run).
	TargetPID uint32
}

// Chromium exe name patterns used to identify browser-owned processes.
var chromiumNames = []string{
	"chrome.exe",
	"chrome",
	"chromium",
	"chrome-headless-shell.exe",
	"chrome_headless_shell",
	"headless_shell",
}

// isChromiumName reports whether a process name looks like a Chromium binary.
func isChromiumName(name string) bool {
	n := strings.ToLower(name)
	for _, c := range chromiumNames {
		if strings.Contains(n, c) {
			return true
		}
	}
	return false
}

// roleFromName guesses the role from the process name alone.
func roleFromName(name string) Role {
	n := strings.ToLower(name)
	if strings.Contains(n, "renderer") || strings.Contains(n, "render") {
		return RoleRenderer
	}
	if strings.Contains(n, "gpu") || strings.Contains(n, "gpu_process") {
		return RoleGPU
	}
	if strings.Contains(n, "utility") {
		return RoleUtility
	}
	return RoleUnknown
}

// Classify assigns a role to every process in the snapshot. The browser
// parent is the topmost chromium-named descendant of the controller; its
// children are classified by name, and anything else under the controller
// tree is auxiliary.
func (o *Ownership) Classify(snap Snapshot) {
	// Build pid -> index and parent relationships.
	byPID := make(map[uint32]int, len(snap.Entries))
	for i := range snap.Entries {
		byPID[snap.Entries[i].PID] = i
	}

	// isOwned: process descends from the controller pid.
	isOwned := func(pid uint32) bool {
		for cur := pid; cur != 0; {
			if cur == o.ControllerPID {
				return true
			}
			idx, ok := byPID[cur]
			if !ok {
				return false
			}
			cur = snap.Entries[idx].PPID
		}
		return false
	}

	for i := range snap.Entries {
		e := &snap.Entries[i]
		// The controller itself.
		if e.PID == o.ControllerPID {
			e.Role = RoleController
			continue
		}
		// The target backend.
		if o.TargetPID != 0 && e.PID == o.TargetPID {
			e.Role = RoleTarget
			continue
		}
		if !isOwned(e.PID) {
			e.Role = RoleUnknown
			continue
		}
		// Chromium-named processes owned by the controller.
		if isChromiumName(e.Name) {
			// The topmost chromium ancestor that is a direct-ish child is the
			// browser parent; if its parent is not chromium-named, it is the
			// browser process itself.
			idx, ok := byPID[e.PPID]
			if !ok || !isChromiumName(snap.Entries[idx].Name) {
				e.Role = RoleBrowser
			} else {
				e.Role = RoleRenderer // default child role; refined below
			}
			continue
		}
		// Children of chromium processes: classify by name.
		if idx, ok := byPID[e.PPID]; ok && isChromiumName(snap.Entries[idx].Name) {
			if r := roleFromName(e.Name); r != RoleUnknown {
				e.Role = r
			} else {
				e.Role = RoleUtility
			}
			continue
		}
		e.Role = RoleAux
	}
}
