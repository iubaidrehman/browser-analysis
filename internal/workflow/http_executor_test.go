package workflow

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// fakeTarget is an in-process stand-in for the synthetic backend.
func fakeTarget(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/session", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"session_id":"sess-1","created_at":"t","expires_at":"t"}`))
	})
	mux.HandleFunc("GET /api/session", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"session_id":"sess-1","created_at":"t","expires_at":"t"}`))
	})
	mux.HandleFunc("POST /api/cart", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"item":{"product_id":"p1","qty":1}}`))
	})
	mux.HandleFunc("POST /api/checkout", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"order":{"id":"ord-1","status":"confirmed"}}`))
	})
	mux.HandleFunc("GET /api/order/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"order":{"id":"ord-1","status":"confirmed"}}`))
	})
	mux.HandleFunc("GET /api/products", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"products":[]}`))
	})
	mux.HandleFunc("GET /api/products/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"product":{"id":"p1"}}`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>ok</body></html>`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestHTTPExecutorComplex(t *testing.T) {
	srv := fakeTarget(t)
	exec := NewHTTPExecutor(srv.Client(), srv.URL)
	wf, _ := Get("complex")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, dur, err := exec.Execute(ctx, wf)
	if err != nil {
		t.Fatalf("execute complex: %v", err)
	}
	if dur <= 0 {
		t.Fatal("expected positive duration")
	}

	// Browser-only ops should be skipped, everything else should be ok.
	var sawSession, sawCart, sawCheckout, sawOrder, sawSkip bool
	for _, r := range results {
		switch r.Op {
		case OpSessionCreate:
			sawSession = r.Status == "ok"
		case OpCartAdd:
			sawCart = r.Status == "ok"
		case OpCheckout:
			sawCheckout = r.Status == "ok"
		case OpOrderGet:
			sawOrder = r.Status == "ok"
		case OpWSConnect, OpLocalStorage, OpIndexedDB, OpExecuteJS:
			sawSkip = sawSkip || r.Status == "skipped"
		}
	}
	if !sawSession || !sawCart || !sawCheckout || !sawOrder {
		t.Fatalf("expected session/cart/checkout/order ops to succeed, got %+v", results)
	}
	if !sawSkip {
		t.Fatal("expected browser-only ops to be skipped")
	}
}

func TestHTTPExecutorUnknownOp(t *testing.T) {
	srv := fakeTarget(t)
	exec := NewHTTPExecutor(srv.Client(), srv.URL)
	ctx := context.Background()

	_, _, err := exec.Execute(ctx, Workflow{Name: "x", Steps: []Step{{Op: "bogus"}}})
	if err == nil {
		t.Fatal("expected error for unknown op")
	}
}
