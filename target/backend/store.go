package main

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type store struct {
	db *sql.DB
}

type session struct {
	ID        string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type product struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	Stock       int     `json:"stock"`
}

type cartItem struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	Qty       int     `json:"qty"`
	Price     float64 `json:"price"`
	LineTotal float64 `json:"line_total"`
}

type order struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Status    string    `json:"status"`
	Total     float64   `json:"total"`
	Items     []cartItem `json:"items"`
	CreatedAt time.Time `json:"created_at"`
}

func openStore(path string) (*store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	s := &store{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	if err := s.seed(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *store) Close() error { return s.db.Close() }

func (s *store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			created_at TEXT NOT NULL,
			expires_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS products (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT NOT NULL,
			price REAL NOT NULL,
			stock INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS cart_items (
			session_id TEXT NOT NULL,
			product_id TEXT NOT NULL,
			qty INTEGER NOT NULL,
			PRIMARY KEY (session_id, product_id)
		)`,
		`CREATE TABLE IF NOT EXISTS orders (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			status TEXT NOT NULL,
			total REAL NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS order_items (
			order_id TEXT NOT NULL,
			product_id TEXT NOT NULL,
			name TEXT NOT NULL,
			qty INTEGER NOT NULL,
			price REAL NOT NULL
		)`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func (s *store) seed() error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM products`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	products := []product{
		{ID: "p1", Name: "Synthetic Widget", Description: "A deterministic widget for benchmarks", Price: 19.99, Stock: 1000},
		{ID: "p2", Name: "Synthetic Gadget", Description: "A deterministic gadget for benchmarks", Price: 49.50, Stock: 1000},
		{ID: "p3", Name: "Synthetic Spanner", Description: "A deterministic spanner for benchmarks", Price: 9.25, Stock: 1000},
	}
	for _, p := range products {
		_, err := s.db.Exec(
			`INSERT INTO products (id, name, description, price, stock) VALUES (?, ?, ?, ?, ?)`,
			p.ID, p.Name, p.Description, p.Price, p.Stock,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *store) CreateSession(ttl time.Duration) (session, error) {
	id := fmt.Sprintf("sess-%d", time.Now().UnixNano())
	now := time.Now().UTC()
	exp := now.Add(ttl)
	_, err := s.db.Exec(`INSERT INTO sessions (id, created_at, expires_at) VALUES (?, ?, ?)`, id, now.Format(time.RFC3339), exp.Format(time.RFC3339))
	return session{ID: id, CreatedAt: now, ExpiresAt: exp}, err
}

func (s *store) GetSession(id string) (session, error) {
	var sess session
	var created, expires string
	err := s.db.QueryRow(`SELECT id, created_at, expires_at FROM sessions WHERE id = ?`, id).Scan(&sess.ID, &created, &expires)
	if err != nil {
		return sess, err
	}
	sess.CreatedAt, _ = time.Parse(time.RFC3339, created)
	sess.ExpiresAt, _ = time.Parse(time.RFC3339, expires)
	return sess, nil
}

func (s *store) ListProducts() []product {
	rows, err := s.db.Query(`SELECT id, name, description, price, stock FROM products ORDER BY id`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []product
	for rows.Next() {
		var p product
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.Stock); err != nil {
			return nil
		}
		out = append(out, p)
	}
	return out
}

func (s *store) GetProduct(id string) (product, error) {
	var p product
	err := s.db.QueryRow(`SELECT id, name, description, price, stock FROM products WHERE id = ?`, id).Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.Stock)
	return p, err
}

func (s *store) AddToCart(sessionID, productID string, qty int) (cartItem, error) {
	p, err := s.GetProduct(productID)
	if err != nil {
		return cartItem{}, err
	}
	if _, err := s.db.Exec(
		`INSERT INTO cart_items (session_id, product_id, qty) VALUES (?, ?, ?)
		 ON CONFLICT(session_id, product_id) DO UPDATE SET qty = qty + excluded.qty`,
		sessionID, productID, qty,
	); err != nil {
		return cartItem{}, err
	}
	return cartItem{ProductID: p.ID, Name: p.Name, Qty: qty, Price: p.Price, LineTotal: p.Price * float64(qty)}, nil
}

func (s *store) GetCart(sessionID string) ([]cartItem, float64) {
	rows, err := s.db.Query(`
		SELECT c.product_id, p.name, c.qty, p.price
		FROM cart_items c JOIN products p ON p.id = c.product_id
		WHERE c.session_id = ?`, sessionID)
	if err != nil {
		return nil, 0
	}
	defer rows.Close()
	var items []cartItem
	var total float64
	for rows.Next() {
		var it cartItem
		if err := rows.Scan(&it.ProductID, &it.Name, &it.Qty, &it.Price); err != nil {
			return nil, 0
		}
		it.LineTotal = it.Price * float64(it.Qty)
		items = append(items, it)
		total += it.LineTotal
	}
	return items, total
}

func (s *store) CreateOrder(sessionID string, items []cartItem) (order, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return order{}, err
	}
	defer tx.Rollback()

	id := fmt.Sprintf("ord-%d", time.Now().UnixNano())
	now := time.Now().UTC()
	var total float64
	for _, it := range items {
		total += it.LineTotal
	}
	if _, err := tx.Exec(`INSERT INTO orders (id, session_id, status, total, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, sessionID, "confirmed", total, now.Format(time.RFC3339)); err != nil {
		return order{}, err
	}
	for _, it := range items {
		if _, err := tx.Exec(`INSERT INTO order_items (order_id, product_id, name, qty, price) VALUES (?, ?, ?, ?, ?)`,
			id, it.ProductID, it.Name, it.Qty, it.Price); err != nil {
			return order{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return order{}, err
	}
	return order{ID: id, SessionID: sessionID, Status: "confirmed", Total: total, Items: items, CreatedAt: now}, nil
}

func (s *store) GetOrder(id string) (order, error) {
	var o order
	var created string
	err := s.db.QueryRow(`SELECT id, session_id, status, total, created_at FROM orders WHERE id = ?`, id).
		Scan(&o.ID, &o.SessionID, &o.Status, &o.Total, &created)
	if err != nil {
		return o, err
	}
	o.CreatedAt, _ = time.Parse(time.RFC3339, created)

	rows, err := s.db.Query(`SELECT product_id, name, qty, price FROM order_items WHERE order_id = ?`, id)
	if err != nil {
		return o, err
	}
	defer rows.Close()
	for rows.Next() {
		var it cartItem
		if err := rows.Scan(&it.ProductID, &it.Name, &it.Qty, &it.Price); err != nil {
			return o, err
		}
		it.LineTotal = it.Price * float64(it.Qty)
		o.Items = append(o.Items, it)
	}
	return o, nil
}

var errNotFound = errors.New("not found")
