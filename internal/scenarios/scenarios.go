// Package scenarios describes the benchmark scenarios available to the CLI.
package scenarios

// Info describes a single benchmark scenario.
type Info struct {
	ID          string
	Description string
	Implemented bool
}

// All returns the full scenario catalogue.
func All() []Info {
	return []Info{
		{ID: "headed", Description: "One headed Chromium process per logical worker.", Implemented: true},
		{ID: "headless", Description: "One headless Chromium process per logical worker.", Implemented: true},
		{ID: "persistent-contexts", Description: "One persistent Chromium with many browser contexts.", Implemented: true},
		{ID: "cdp", Description: "One persistent Chromium controlled through CDP.", Implemented: false},
		{ID: "http", Description: "Lightweight HTTP workers without a browser.", Implemented: true},
		{ID: "hybrid", Description: "HTTP workers that escalate selected operations to a browser.", Implemented: false},
		{ID: "cef", Description: "Off-screen CEF/Chromium (optional).", Implemented: false},
	}
}
