package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestApp(t *testing.T) *app {
	t.Helper()
	dir := t.TempDir()
	store, err := openStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	cfg := targetConfig{
		addr:              ":0",
		apiLatencyMs:      0,
		payloadKB:         0,
		jsWorkloadUnits:   0,
		sessionTTLSeconds: 3600,
	}
	a := &app{cfg: cfg, store: store, hub: newHub(), rng: newSeededRand(42)}
	go a.hub.run()
	a.routes()
	return a
}

func doReq(t *testing.T, a *app, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = strings.NewReader(string(b))
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	a.mux.ServeHTTP(rec, req)
	return rec
}

func TestSessionLifecycle(t *testing.T) {
	a := newTestApp(t)

	rec := doReq(t, a, http.MethodGet, "/api/session", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("create session: got %d", rec.Code)
	}
	var sess sessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &sess); err != nil {
		t.Fatal(err)
	}
	if sess.SessionID == "" {
		t.Fatal("expected non-empty session id")
	}

	rec = doReq(t, a, http.MethodGet, "/api/session?id="+sess.SessionID, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("resume session: got %d", rec.Code)
	}
}

func TestProducts(t *testing.T) {
	a := newTestApp(t)
	rec := doReq(t, a, http.MethodGet, "/api/products", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("products: got %d", rec.Code)
	}
	var resp struct {
		Products []product `json:"products"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Products) != 3 {
		t.Fatalf("expected 3 seeded products, got %d", len(resp.Products))
	}
}

func TestCheckoutFlow(t *testing.T) {
	a := newTestApp(t)

	rec := doReq(t, a, http.MethodGet, "/api/session", nil)
	var sess sessionResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &sess)

	rec = doReq(t, a, http.MethodPost, "/api/cart", map[string]any{
		"session_id": sess.SessionID, "product_id": "p1", "qty": 2,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("add to cart: got %d", rec.Code)
	}

	rec = doReq(t, a, http.MethodGet, "/api/cart?session_id="+sess.SessionID, nil)
	var cart struct {
		Items []cartItem `json:"items"`
		Total float64    `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &cart); err != nil {
		t.Fatal(err)
	}
	if len(cart.Items) != 1 || cart.Items[0].Qty != 2 {
		t.Fatalf("unexpected cart: %+v", cart.Items)
	}
	if cart.Total != 39.98 {
		t.Fatalf("total = %v, want 39.98", cart.Total)
	}

	rec = doReq(t, a, http.MethodPost, "/api/checkout", map[string]any{"session_id": sess.SessionID})
	if rec.Code != http.StatusOK {
		t.Fatalf("checkout: got %d", rec.Code)
	}
	var checkout struct {
		Order order `json:"order"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &checkout); err != nil {
		t.Fatal(err)
	}
	if checkout.Order.Status != "confirmed" {
		t.Fatalf("status = %q, want confirmed", checkout.Order.Status)
	}

	rec = doReq(t, a, http.MethodGet, "/api/order/"+checkout.Order.ID, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get order: got %d", rec.Code)
	}
}

func TestCartEmptyCheckoutRejected(t *testing.T) {
	a := newTestApp(t)
	rec := doReq(t, a, http.MethodGet, "/api/session", nil)
	var sess sessionResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &sess)

	rec = doReq(t, a, http.MethodPost, "/api/checkout", map[string]any{"session_id": sess.SessionID})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty cart checkout: got %d, want 400", rec.Code)
	}
}

func TestArtificialLatency(t *testing.T) {
	a := newTestApp(t)
	a.cfg.apiLatencyMs = 50
	start := time.Now()
	rec := doReq(t, a, http.MethodGet, "/api/products", nil)
	elapsed := time.Since(start)
	if rec.Code != http.StatusOK {
		t.Fatalf("products: got %d", rec.Code)
	}
	if elapsed < 40*time.Millisecond {
		t.Fatalf("latency not applied: %v", elapsed)
	}
}
