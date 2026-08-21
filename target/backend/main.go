package main

import (
	"encoding/json"
	"flag"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"
)

// targetConfig holds the tunable synthetic workload parameters. All of them
// can be set via environment variables so a benchmark can shape latency,
// payload size, and JS workload without rebuilding.
type targetConfig struct {
	addr               string
	dbPath             string
	apiLatencyMs       int
	payloadKB          int
	jsWorkloadUnits    int
	maxOrders          int
	seed               int64
	sessionTTLSeconds  int
}

func loadConfig() targetConfig {
	return targetConfig{
		addr:              envOr("TARGET_ADDR", ":8080"),
		dbPath:            envOr("TARGET_DB", "target.db"),
		apiLatencyMs:      envIntOr("TARGET_API_LATENCY_MS", 0),
		payloadKB:         envIntOr("TARGET_PAYLOAD_KB", 4),
		jsWorkloadUnits:   envIntOr("TARGET_JS_WORKLOAD_UNITS", 0),
		maxOrders:         envIntOr("TARGET_MAX_ORDERS", 1000),
		seed:              int64(envIntOr("TARGET_SEED", 42)),
		sessionTTLSeconds: envIntOr("TARGET_SESSION_TTL_SECONDS", 3600),
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func newSeededRand(seed int64) *rand.Rand {
	return rand.New(rand.NewSource(seed))
}

func envIntOr(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func main() {
	cfg := loadConfig()
	flag.StringVar(&cfg.addr, "addr", cfg.addr, "listen address")
	flag.StringVar(&cfg.dbPath, "db", cfg.dbPath, "sqlite database path")
	flag.Parse()

	store, err := openStore(cfg.dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer store.Close()

	hub := newHub()
	go hub.run()

	app := &app{cfg: cfg, store: store, hub: hub, rng: rand.New(rand.NewSource(cfg.seed))}
	app.routes()

	log.Printf("target listening on %s (latency=%dms payload=%dKB js=%d units db=%s)",
		cfg.addr, cfg.apiLatencyMs, cfg.payloadKB, cfg.jsWorkloadUnits, cfg.dbPath)
	if err := http.ListenAndServe(cfg.addr, app.mux); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

// app bundles the target application's dependencies and routes.
type app struct {
	cfg   targetConfig
	store *store
	hub   *hub
	rng   *rand.Rand
	mux   http.Handler
}

func (a *app) routes() {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/session", a.handleSession)
	mux.HandleFunc("GET /api/session", a.handleSessionGet)
	mux.HandleFunc("GET /api/products", a.handleProducts)
	mux.HandleFunc("GET /api/products/{id}", a.handleProduct)
	mux.HandleFunc("POST /api/cart", a.handleCartAdd)
	mux.HandleFunc("GET /api/cart", a.handleCartGet)
	mux.HandleFunc("POST /api/checkout", a.handleCheckout)
	mux.HandleFunc("GET /api/order/{id}", a.handleOrder)
	mux.HandleFunc("GET /ws/events", a.handleWS)
	mux.HandleFunc("/", a.handleIndex)

	a.mux = withMiddleware(mux, a)
}

// handleIndex serves a minimal HTML document so HTTP-only workers can emulate
// a page navigation without depending on the frontend being up.
func (a *app) handleIndex(w http.ResponseWriter, r *http.Request) {
	a.syntheticDelay()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte("<!doctype html><html><head><title>BCRL Target</title></head><body><h1>BCRL Synthetic Target</h1></body></html>"))
}

// ---- shared helpers ----

func (a *app) syntheticDelay() {
	if a.cfg.apiLatencyMs > 0 {
		time.Sleep(time.Duration(a.cfg.apiLatencyMs) * time.Millisecond)
	}
}

// payload returns a deterministic random string of exactly n bytes using
// printable ASCII so the JSON wire size matches the configured payload size.
func (a *app) payload(n int) string {
	if n <= 0 {
		return "{}"
	}
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	out := make([]byte, n)
	for i := range out {
		out[i] = alphabet[a.rng.Intn(len(alphabet))]
	}
	return string(out)
}

// checksum burns CPU proportionally to units using a non-elidable hash loop.
func (a *app) checksum(units int) uint64 {
	var h uint64 = 1469598103934665603 // FNV offset basis
	for i := 0; i < units; i++ {
		h ^= uint64(i)
		h *= 1099511628211 // FNV prime
	}
	return h
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	return dec.Decode(v)
}

// ---- handlers ----

type sessionResponse struct {
	SessionID string    `json:"session_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// handleSessionGet resumes an existing session if the id query parameter is
// present; without one it is a read-only 404 so GET stays side-effect free.
func (a *app) handleSessionGet(w http.ResponseWriter, r *http.Request) {
	a.syntheticDelay()
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing session id"})
		return
	}
	s, err := a.store.GetSession(id)
	if err != nil || time.Now().After(s.ExpiresAt) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, sessionResponse{SessionID: s.ID, CreatedAt: s.CreatedAt, ExpiresAt: s.ExpiresAt})
}

func (a *app) handleSession(w http.ResponseWriter, r *http.Request) {
	a.syntheticDelay()

	ttl := time.Duration(a.cfg.sessionTTLSeconds) * time.Second
	now := time.Now().UTC()

	s, err := a.store.CreateSession(ttl)
	if err != nil {
		log.Printf("session create error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	a.hub.broadcast(event{Type: "session", SessionID: s.ID})
	writeJSON(w, http.StatusOK, sessionResponse{SessionID: s.ID, CreatedAt: s.CreatedAt, ExpiresAt: now.Add(ttl)})
}

func (a *app) handleProducts(w http.ResponseWriter, r *http.Request) {
	a.syntheticDelay()
	products := a.store.ListProducts()
	_ = r
	writeJSON(w, http.StatusOK, map[string]any{"products": products, "payload": a.payload(a.cfg.payloadKB * 1024)})
}

func (a *app) handleProduct(w http.ResponseWriter, r *http.Request) {
	a.syntheticDelay()
	id := r.PathValue("id")
	p, err := a.store.GetProduct(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "product not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"product": p, "payload": a.payload(a.cfg.payloadKB * 1024)})
}

func (a *app) handleCartAdd(w http.ResponseWriter, r *http.Request) {
	a.syntheticDelay()
	var req struct {
		SessionID string `json:"session_id"`
		ProductID string `json:"product_id"`
		Qty       int    `json:"qty"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	if req.Qty <= 0 {
		req.Qty = 1
	}
	if _, err := a.store.GetProduct(req.ProductID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "product not found"})
		return
	}
	item, err := a.store.AddToCart(req.SessionID, req.ProductID, req.Qty)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	a.hub.broadcast(event{Type: "cart", SessionID: req.SessionID})
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (a *app) handleCartGet(w http.ResponseWriter, r *http.Request) {
	a.syntheticDelay()
	sessionID := r.URL.Query().Get("session_id")
	items, total := a.store.GetCart(sessionID)
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (a *app) handleCheckout(w http.ResponseWriter, r *http.Request) {
	a.syntheticDelay()
	var req struct {
		SessionID string `json:"session_id"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	items, _ := a.store.GetCart(req.SessionID)
	if len(items) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cart is empty"})
		return
	}

	// Deterministic CPU workload: a checksum loop proportional to the
	// configured unit count. The result is written into the response so the
	// compiler cannot elide the work.
	checksum := a.checksum(a.cfg.jsWorkloadUnits)

	order, err := a.store.CreateOrder(req.SessionID, items)
	if err != nil {
		log.Printf("checkout error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	a.hub.broadcast(event{Type: "order", SessionID: req.SessionID, OrderID: order.ID})
	writeJSON(w, http.StatusOK, map[string]any{"order": order, "checksum": checksum})
}

func (a *app) handleOrder(w http.ResponseWriter, r *http.Request) {
	a.syntheticDelay()
	id := r.PathValue("id")
	o, err := a.store.GetOrder(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "order not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"order": o})
}

// ---- middleware ----

func withMiddleware(next http.Handler, a *app) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
