package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

type event struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id,omitempty"`
	OrderID   string `json:"order_id,omitempty"`
	At        int64  `json:"at"`
}

type hub struct {
	clients map[*client]bool
	join    chan *client
	leave   chan *client
	events  chan event
}

type client struct {
	conn *websocket.Conn
	send chan []byte
}

func newHub() *hub {
	return &hub{
		clients: make(map[*client]bool),
		join:    make(chan *client),
		leave:   make(chan *client),
		events:  make(chan event, 256),
	}
}

func (h *hub) run() {
	for {
		select {
		case c := <-h.join:
			h.clients[c] = true
		case c := <-h.leave:
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
		case ev := <-h.events:
			ev.At = time.Now().UnixMilli()
			payload, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			for c := range h.clients {
				select {
				case c.send <- payload:
				default:
					// Slow client: drop it rather than blocking the hub.
					delete(h.clients, c)
					close(c.send)
				}
			}
		}
	}
}

func (h *hub) broadcast(ev event) {
	select {
	case h.events <- ev:
	default:
	}
}

func (a *app) handleWS(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade: %v", err)
		return
	}
	c := &client{conn: conn, send: make(chan []byte, 16)}
	a.hub.join <- c
	defer func() { a.hub.leave <- c }()

	go func() {
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				_ = conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ping.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
