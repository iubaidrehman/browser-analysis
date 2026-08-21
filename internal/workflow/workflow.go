// Package workflow defines the synthetic workflows the benchmark executes and
// the executor that runs them against the target application.
//
// The four workflows (minimal, SPA, stateful, complex) are expressed as
// generic steps. Each executor maps those steps onto its transport: the HTTP
// executor calls the target's REST API directly; the browser executors
// (later phases) drive real pages.
package workflow

import (
	"errors"
	"net"
	"strings"
	"time"
)

// OpType is a single workflow operation.
type OpType string

const (
	OpLaunch        OpType = "launch"
	OpNavigate      OpType = "navigate"
	OpDOMReady      OpType = "dom_ready"
	OpSessionCreate OpType = "session_create"
	OpProducts      OpType = "products"
	OpProduct       OpType = "product"
	OpCartAdd       OpType = "cart_add"
	OpCartGet       OpType = "cart_get"
	OpCheckout      OpType = "checkout"
	OpOrderGet      OpType = "order_get"
	OpLocalStorage  OpType = "local_storage"
	OpIndexedDB     OpType = "indexed_db"
	OpExecuteJS     OpType = "execute_js"
	OpWSConnect     OpType = "ws_connect"
)

// Step is one operation in a workflow.
type Step struct {
	Op        OpType `json:"op"`
	ProductID string `json:"product_id,omitempty"`
	Qty       int    `json:"qty,omitempty"`
}

// Workflow is a named, ordered sequence of steps.
type Workflow struct {
	Name  string `json:"name"`
	Steps []Step `json:"steps"`
}

// RequestInfo describes a single HTTP request issued while executing a step.
type RequestInfo struct {
	Method    string
	Path      string
	Status    int
	Duration  time.Duration
	ErrString string
}

// Result is the outcome of one workflow step.
type Result struct {
	Op       OpType
	Duration time.Duration
	Status   string // "ok", "skipped", "error"
	Request  *RequestInfo
	Error    error
}

// Error classification (spec section 29).
const (
	ErrTypeApp             = "application_error"
	ErrTypeBrowser         = "browser_error"
	ErrTypeTimeout         = "timeout"
	ErrTypeResource        = "resource_exhaustion"
	ErrTypeCancellation    = "cancellation"
	ErrTypeInfrastructure  = "infrastructure_error"
)

// ClassifyError maps an error to one of the spec's error types.
func ClassifyError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, contextDeadline) || strings.Contains(err.Error(), "deadline exceeded") {
		return ErrTypeTimeout
	}
	if errors.Is(err, contextCancelled) {
		return ErrTypeCancellation
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return ErrTypeInfrastructure
	}
	if strings.Contains(err.Error(), "connection refused") ||
		strings.Contains(err.Error(), "no such host") ||
		strings.Contains(err.Error(), "EOF") {
		return ErrTypeInfrastructure
	}
	return ErrTypeApp
}
