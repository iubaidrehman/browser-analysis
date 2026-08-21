package hybrid

import (
	"testing"

	"bcrl/internal/workflow"
)

func TestPolicyWorkflowRouting(t *testing.T) {
	p := PolicyWorkflow
	cases := []struct {
		op          workflow.OpType
		wantBrowser bool
	}{
		{workflow.OpNavigate, true},
		{workflow.OpDOMReady, true},
		{workflow.OpLocalStorage, true},
		{workflow.OpIndexedDB, true},
		{workflow.OpExecuteJS, true},
		{workflow.OpWSConnect, true},
		{workflow.OpSessionCreate, false},
		{workflow.OpProducts, false},
		{workflow.OpProduct, false},
		{workflow.OpCartAdd, false},
		{workflow.OpCheckout, false},
		{workflow.OpOrderGet, false},
		{workflow.OpLaunch, false},
	}
	for _, c := range cases {
		if got := p.BrowserOnly(c.op); got != c.wantBrowser {
			t.Errorf("op %s: browserOnly = %v, want %v", c.op, got, c.wantBrowser)
		}
	}
}

// TestComplexWorkflowEscalationCount verifies the complex workflow produces
// exactly 5 escalations (navigate, execute_js, local_storage, indexed_db,
// ws_connect).
func TestComplexWorkflowEscalationCount(t *testing.T) {
	wf, ok := workflow.Get("complex")
	if !ok {
		t.Fatal("complex workflow missing")
	}
	p := PolicyWorkflow
	n := 0
	for _, step := range wf.Steps {
		if p.BrowserOnly(step.Op) {
			n++
		}
	}
	if n != 5 {
		t.Fatalf("complex workflow escalations = %d, want 5", n)
	}
}
