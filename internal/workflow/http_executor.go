package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPExecutor runs workflows against the target REST API using a shared
// http.Client with connection pooling and keep-alive. It maps each workflow
// step onto the equivalent API call (spec section 5, scenario E).
type HTTPExecutor struct {
	client  *http.Client
	baseURL string
}

// NewHTTPExecutor builds an executor backed by the given client.
func NewHTTPExecutor(client *http.Client, baseURL string) *HTTPExecutor {
	return &HTTPExecutor{client: client, baseURL: strings.TrimSuffix(baseURL, "/")}
}

// Execute runs the workflow and returns a result per step plus the overall
// duration. Steps that are inherently browser-only (localStorage, IndexedDB,
// execute_js, ws_connect) are skipped by the HTTP executor and recorded with
// status "skipped".
func (e *HTTPExecutor) Execute(ctx context.Context, wf Workflow) ([]Result, time.Duration, error) {
	start := time.Now()
	sessionID := ""
	orderID := ""
	results := make([]Result, 0, len(wf.Steps))

	for _, step := range wf.Steps {
		res, err := e.executeStep(ctx, step, &sessionID, &orderID)
		results = append(results, res)
		if err != nil {
			return results, time.Since(start), err
		}
	}
	return results, time.Since(start), nil
}

func (e *HTTPExecutor) executeStep(ctx context.Context, step Step, sessionID, orderID *string) (Result, error) {
	switch step.Op {
	case OpLaunch:
		// No browser launch in the HTTP architecture; nothing to do.
		return Result{Op: step.Op, Duration: 0, Status: "ok"}, nil

	case OpSessionCreate:
		return e.do(ctx, step.Op, http.MethodPost, "/api/session", nil, func(body []byte) error {
			var v struct {
				SessionID string `json:"session_id"`
			}
			if err := json.Unmarshal(body, &v); err != nil {
				return err
			}
			*sessionID = v.SessionID
			return nil
		})

	case OpNavigate, OpDOMReady:
		// HTTP workers don't navigate a real page; the equivalent is fetching
		// the target document served by the backend.
		return e.do(ctx, step.Op, http.MethodGet, "/", nil, nil)

	case OpProducts:
		return e.do(ctx, step.Op, http.MethodGet, "/api/products", nil, nil)

	case OpProduct:
		return e.do(ctx, step.Op, http.MethodGet, "/api/products/"+step.ProductID, nil, nil)

	case OpCartAdd:
		if *sessionID == "" {
			return Result{Op: step.Op, Duration: 0, Status: "error", Error: fmt.Errorf("no session")}, fmt.Errorf("no session")
		}
		body := map[string]any{"session_id": *sessionID, "product_id": step.ProductID, "qty": step.Qty}
		return e.do(ctx, step.Op, http.MethodPost, "/api/cart", body, nil)

	case OpCartGet:
		if *sessionID == "" {
			return Result{Op: step.Op, Duration: 0, Status: "error", Error: fmt.Errorf("no session")}, fmt.Errorf("no session")
		}
		return e.do(ctx, step.Op, http.MethodGet, "/api/cart?session_id="+*sessionID, nil, nil)

	case OpCheckout:
		if *sessionID == "" {
			return Result{Op: step.Op, Duration: 0, Status: "error", Error: fmt.Errorf("no session")}, fmt.Errorf("no session")
		}
		body := map[string]any{"session_id": *sessionID}
		return e.do(ctx, step.Op, http.MethodPost, "/api/checkout", body, func(b []byte) error {
			var v struct {
				Order struct {
					ID string `json:"id"`
				} `json:"order"`
			}
			if err := json.Unmarshal(b, &v); err != nil {
				return err
			}
			*orderID = v.Order.ID
			return nil
		})

	case OpOrderGet:
		if *orderID == "" {
			return Result{Op: step.Op, Duration: 0, Status: "error", Error: fmt.Errorf("no order")}, fmt.Errorf("no order")
		}
		return e.do(ctx, step.Op, http.MethodGet, "/api/order/"+*orderID, nil, nil)

	case OpLocalStorage, OpIndexedDB, OpExecuteJS, OpWSConnect:
		// Browser-only operations; the HTTP baseline skips them.
		return Result{Op: step.Op, Duration: 0, Status: "skipped"}, nil
	}
	return Result{Op: step.Op, Status: "error", Error: fmt.Errorf("unknown op %q", step.Op)}, fmt.Errorf("unknown op %q", step.Op)
}

func (e *HTTPExecutor) do(ctx context.Context, op OpType, method, path string, body any, extract func([]byte) error) (Result, error) {
	start := time.Now()

	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return Result{Op: op, Duration: 0, Status: "error", Error: err}, err
		}
		rd = strings.NewReader(string(b))
	}

	req, err := http.NewRequestWithContext(ctx, method, e.baseURL+path, rd)
	if err != nil {
		return Result{Op: op, Duration: 0, Status: "error", Error: err}, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := e.client.Do(req)
	if err != nil {
		d := time.Since(start)
		return Result{Op: op, Duration: d, Status: "error", Request: &RequestInfo{Method: method, Path: path, ErrString: err.Error()}, Error: err}, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	d := time.Since(start)
	reqInfo := &RequestInfo{Method: method, Path: path, Status: resp.StatusCode, Duration: d}

	if err != nil {
		return Result{Op: op, Duration: d, Status: "error", Request: reqInfo, Error: err}, err
	}
	if resp.StatusCode >= 400 {
		// Include the server's error message so failure classification is
		// actionable (e.g. a SQLite "database is locked" vs a 400 cart empty).
		bodyText := strings.TrimSpace(string(raw))
		if len(bodyText) > 200 {
			bodyText = bodyText[:200]
		}
		err := fmt.Errorf("http %d on %s %s: %s", resp.StatusCode, method, path, bodyText)
		reqInfo.ErrString = err.Error()
		return Result{Op: op, Duration: d, Status: "error", Request: reqInfo, Error: err}, err
	}
	if extract != nil {
		if err := extract(raw); err != nil {
			return Result{Op: op, Duration: d, Status: "error", Request: reqInfo, Error: err}, err
		}
	}
	return Result{Op: op, Duration: d, Status: "ok", Request: reqInfo}, nil
}
