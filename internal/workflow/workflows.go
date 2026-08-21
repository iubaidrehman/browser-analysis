package workflow

// DefaultWorkflows returns the four benchmark workflows (spec section 6).
// Each is deterministic: fixed step order and fixed product quantities.
func DefaultWorkflows() []Workflow {
	return []Workflow{
		{
			Name: "minimal",
			Steps: []Step{
				{Op: OpLaunch},
				{Op: OpNavigate, ProductID: "/home"},
				{Op: OpDOMReady},
			},
		},
		{
			Name: "SPA",
			Steps: []Step{
				{Op: OpLaunch},
				{Op: OpNavigate, ProductID: "/home"},
				{Op: OpNavigate, ProductID: "/product/p1"},
				{Op: OpNavigate, ProductID: "/cart"},
			},
		},
		{
			Name: "stateful",
			Steps: []Step{
				{Op: OpLaunch},
				{Op: OpSessionCreate},
				{Op: OpNavigate, ProductID: "/home"},
				{Op: OpLocalStorage},
				{Op: OpIndexedDB},
				{Op: OpProducts},
			},
		},
		{
			Name: "complex",
			Steps: []Step{
				{Op: OpLaunch},
				{Op: OpSessionCreate},
				{Op: OpNavigate, ProductID: "/home"},
				{Op: OpExecuteJS},
				{Op: OpLocalStorage},
				{Op: OpIndexedDB},
				{Op: OpProducts},
				{Op: OpWSConnect},
				{Op: OpCartAdd, ProductID: "p1", Qty: 1},
				{Op: OpCheckout},
				{Op: OpOrderGet},
			},
		},
	}
}

// Get returns a named workflow from DefaultWorkflows.
func Get(name string) (Workflow, bool) {
	for _, w := range DefaultWorkflows() {
		if w.Name == name {
			return w, true
		}
	}
	return Workflow{}, false
}
