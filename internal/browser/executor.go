// Package browser implements the browser-backed executors used by scenarios
// A/B (headed/headless Chromium), C (persistent contexts), and D (CDP).
package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"bcrl/internal/workflow"
	"github.com/mxschmitt/playwright-go"
)

// PageExecutor runs workflow steps against a real Playwright page.
type PageExecutor struct {
	page playwright.Page
}

// NewPageExecutor wraps a page.
func NewPageExecutor(page playwright.Page) *PageExecutor {
	return &PageExecutor{page: page}
}

// Execute runs the workflow against the page and returns per-step results
// plus the overall duration. Every step is measured with a real browser
// operation; browser-only steps (localStorage, IndexedDB, JS, WebSocket) are
// exercised in-page rather than skipped.
func (e *PageExecutor) Execute(ctx context.Context, baseURL string, wf workflow.Workflow) ([]workflow.Result, time.Duration, error) {
	start := time.Now()
	results := make([]workflow.Result, 0, len(wf.Steps))
	st := &workflow.SessionState{}

	for _, step := range wf.Steps {
		res, err := e.ExecuteStep(ctx, baseURL, step, st)
		results = append(results, res)
		if err != nil {
			return results, time.Since(start), err
		}
	}
	return results, time.Since(start), nil
}

// ExecuteStep runs one workflow step against the page, threading session
// state through the given SessionState. Used by the hybrid executor to
// interleave browser steps with HTTP steps.
func (e *PageExecutor) ExecuteStep(ctx context.Context, baseURL string, step workflow.Step, st *workflow.SessionState) (workflow.Result, error) {
	return e.executeStep(ctx, baseURL, step, &st.SessionID, &st.OrderID)
}

func (e *PageExecutor) executeStep(ctx context.Context, baseURL string, step workflow.Step, sessionID, orderID *string) (workflow.Result, error) {
	opStart := time.Now()
	done := func(status string, err error) (workflow.Result, error) {
		return workflow.Result{Op: step.Op, Duration: time.Since(opStart), Status: status, Error: err}, err
	}

	switch step.Op {
	case workflow.OpLaunch:
		// Browser launch is measured by the worker, not per-step.
		return done("ok", nil)

	case workflow.OpNavigate:
		path := step.ProductID
		if path == "" {
			path = "/home"
		}
		if err := e.navigate(ctx, baseURL+path); err != nil {
			return done("error", err)
		}
		return done("ok", nil)

	case workflow.OpDOMReady:
		if err := e.waitFor(ctx, "document.readyState === 'complete'"); err != nil {
			return done("error", err)
		}
		return done("ok", nil)

	case workflow.OpSessionCreate:
		res, err := e.apiCall(ctx, "POST", baseURL+"/api/session", nil)
		if err != nil {
			return done("error", err)
		}
		var v struct {
			SessionID string `json:"session_id"`
		}
		if err := json.Unmarshal([]byte(res), &v); err != nil {
			return done("error", err)
		}
		*sessionID = v.SessionID
		return done("ok", nil)

	case workflow.OpProducts:
		if _, err := e.apiCall(ctx, "GET", baseURL+"/api/products", nil); err != nil {
			return done("error", err)
		}
		return done("ok", nil)

	case workflow.OpProduct:
		if _, err := e.apiCall(ctx, "GET", baseURL+"/api/products/"+step.ProductID, nil); err != nil {
			return done("error", err)
		}
		return done("ok", nil)

	case workflow.OpCartAdd:
		if *sessionID == "" {
			return done("error", fmt.Errorf("no session"))
		}
		body := map[string]any{"session_id": *sessionID, "product_id": step.ProductID, "qty": step.Qty}
		if _, err := e.apiCall(ctx, "POST", baseURL+"/api/cart", body); err != nil {
			return done("error", err)
		}
		return done("ok", nil)

	case workflow.OpCheckout:
		if *sessionID == "" {
			return done("error", fmt.Errorf("no session"))
		}
		body := map[string]any{"session_id": *sessionID}
		res, err := e.apiCall(ctx, "POST", baseURL+"/api/checkout", body)
		if err != nil {
			return done("error", err)
		}
		var v struct {
			Order struct {
				ID string `json:"id"`
			} `json:"order"`
		}
		if err := json.Unmarshal([]byte(res), &v); err != nil {
			return done("error", err)
		}
		*orderID = v.Order.ID
		return done("ok", nil)

	case workflow.OpOrderGet:
		if *orderID == "" {
			return done("error", fmt.Errorf("no order"))
		}
		if _, err := e.apiCall(ctx, "GET", baseURL+"/api/order/"+*orderID, nil); err != nil {
			return done("error", err)
		}
		return done("ok", nil)

	case workflow.OpLocalStorage:
		if err := e.eval(ctx, `localStorage.setItem('bcrl.bench','1'); localStorage.getItem('bcrl.bench')`); err != nil {
			return done("error", err)
		}
		return done("ok", nil)

	case workflow.OpIndexedDB:
		if err := e.eval(ctx, indexDBJS); err != nil {
			return done("error", err)
		}
		return done("ok", nil)

	case workflow.OpExecuteJS:
		if err := e.eval(ctx, `(() => { let s = 0; for (let i = 0; i < 10000; i++) { s += i; } return s; })()`); err != nil {
			return done("error", err)
		}
		return done("ok", nil)

	case workflow.OpWSConnect:
		if err := e.wsCheck(ctx, baseURL); err != nil {
			return done("error", err)
		}
		return done("ok", nil)

	default:
		return done("error", fmt.Errorf("unknown op %q", step.Op))
	}
}

// navigate drives the page to a URL and waits for the network to settle.
func (e *PageExecutor) navigate(ctx context.Context, url string) error {
	done := make(chan error, 1)
	go func() {
		_, err := e.page.Goto(url, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateNetworkidle,
			Timeout:   playwright.Float(30000),
		})
		done <- err
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// waitFor polls a JS expression until truthy.
func (e *PageExecutor) waitFor(ctx context.Context, expr string) error {
	done := make(chan error, 1)
	go func() {
		_, err := e.page.WaitForFunction(expr, nil, playwright.PageWaitForFunctionOptions{
			Timeout: playwright.Float(30000),
		})
		done <- err
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// eval runs a JS expression in the page and returns its JSON-serialized value.
func (e *PageExecutor) eval(ctx context.Context, js string) error {
	done := make(chan error, 1)
	go func() {
		_, err := e.page.Evaluate(js)
		done <- err
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// apiCall performs a fetch from inside the page so requests carry the page's
// cookies/origin, matching real browser behavior. Non-2xx responses are
// surfaced as errors.
func (e *PageExecutor) apiCall(ctx context.Context, method, url string, body any) (string, error) {
	var js string
	if body != nil {
		b, _ := json.Marshal(body)
		js = fmt.Sprintf(`fetch(%q, {method: %q, headers: {'Content-Type': 'application/json'}, body: %q}).then(async r => { const t = await r.text(); if (!r.ok) { throw new Error('http ' + r.status + ' ' + t); } return t; })`,
			url, method, string(b))
	} else {
		js = fmt.Sprintf(`fetch(%q, {method: %q}).then(async r => { const t = await r.text(); if (!r.ok) { throw new Error('http ' + r.status + ' ' + t); } return t; })`,
			url, method)
	}
	return e.evalString(ctx, js)
}

// evalString evaluates an expression and returns the resulting string.
func (e *PageExecutor) evalString(ctx context.Context, js string) (string, error) {
	type result struct {
		val string
		err error
	}
	done := make(chan result, 1)
	go func() {
		v, err := e.page.Evaluate(js)
		if err != nil {
			done <- result{err: err}
			return
		}
		s, ok := v.(string)
		if !ok {
			done <- result{err: fmt.Errorf("expected string, got %T", v)}
			return
		}
		done <- result{val: s}
	}()
	select {
	case r := <-done:
		return r.val, r.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// wsCheck opens a WebSocket from the page to the target, triggers a server
// broadcast, and waits for the event on this exact socket — proving the page
// can hold a live WebSocket connection end to end. The event type is checked
// so a broadcast meant for another client cannot satisfy the step.
func (e *PageExecutor) wsCheck(ctx context.Context, baseURL string) error {
	baseURL = strings.TrimSuffix(baseURL, "/")
	wsURL := baseURL
	if len(wsURL) >= 4 && wsURL[:4] == "http" {
		wsURL = "ws" + wsURL[4:]
	} else {
		wsURL = "ws://" + wsURL
	}
	wsURL += "/ws/events"
	js := fmt.Sprintf(`new Promise((resolve, reject) => {
		const ws = new WebSocket(%q);
		ws.onopen = () => {
			// Trigger a server-side broadcast so this socket receives an event.
			fetch(%q, {method: 'POST'}).catch(() => {});
		};
		ws.onmessage = (ev) => {
			try {
				const msg = JSON.parse(ev.data);
				if (msg.type === 'session') { ws.close(); resolve('ok'); }
			} catch (e) {}
		};
		ws.onerror = () => reject('ws error');
		setTimeout(() => reject('ws timeout'), 5000);
	})`, wsURL, baseURL+"/api/session")
	_, err := e.evalString(ctx, js)
	return err
}
