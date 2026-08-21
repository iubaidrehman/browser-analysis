// Package hybrid implements scenario F: logical workers execute HTTP steps
// over HTTP and browser-only steps (localStorage, IndexedDB, execute-JS,
// WebSocket) in a browser context, sharing session state across transports.
package hybrid

import (
	"context"
	"time"

	"bcrl/internal/browser"
	"bcrl/internal/workflow"
)

// EscalationPolicy selects which steps escalate to the browser.
type EscalationPolicy string

const (
	// PolicyWorkflow escalates browser-only ops plus navigation; API ops stay
	// on HTTP. This is the only implemented policy.
	PolicyWorkflow EscalationPolicy = "workflow"
)

// BrowserOnly reports whether an op must run in a browser under the workflow
// policy. Navigation is routed to the browser too, because browser-only
// storage ops (localStorage, IndexedDB) require the page to be on an http
// origin rather than about:blank.
func (p EscalationPolicy) BrowserOnly(op workflow.OpType) bool {
	switch p {
	case PolicyWorkflow:
		switch op {
		case workflow.OpLocalStorage, workflow.OpIndexedDB, workflow.OpExecuteJS, workflow.OpWSConnect:
			return true
		case workflow.OpNavigate, workflow.OpDOMReady:
			return true
		}
	}
	return false
}

// Executor routes each workflow step to the appropriate transport. Session
// state is shared so an HTTP session can be used by browser steps and vice
// versa.
type Executor struct {
	http    *workflow.HTTPExecutor
	page    *browser.PageExecutor
	baseURL string
	policy  EscalationPolicy
}

// NewExecutor builds a hybrid executor over the given transports.
func NewExecutor(policy EscalationPolicy, httpExec *workflow.HTTPExecutor, pageExec *browser.PageExecutor, baseURL string) *Executor {
	return &Executor{http: httpExec, page: pageExec, baseURL: baseURL, policy: policy}
}

// Execute runs the workflow, routing each step by transport. Browser-only
// steps count as escalations.
func (e *Executor) Execute(ctx context.Context, wf workflow.Workflow) ([]workflow.Result, time.Duration, int, error) {
	start := time.Now()
	st := &workflow.SessionState{}
	results := make([]workflow.Result, 0, len(wf.Steps))
	escalations := 0

	for _, step := range wf.Steps {
		var (
			res workflow.Result
			err error
		)
		if e.policy.BrowserOnly(step.Op) {
			res, err = e.page.ExecuteStep(ctx, e.baseURL, step, st)
			if err == nil {
				escalations++
			}
		} else {
			res, err = e.http.ExecuteStep(ctx, step, st)
		}
		results = append(results, res)
		if err != nil {
			return results, time.Since(start), escalations, err
		}
	}
	return results, time.Since(start), escalations, nil
}
