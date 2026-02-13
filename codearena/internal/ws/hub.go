// Package ws implements the websocket hub that pushes live run events to
// connected users.
package ws

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	pingInterval = 30 * time.Second
	pongWait     = pingInterval + 15*time.Second
	writeWait    = 10 * time.Second
	sendBuffer   = 16
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Dev-friendly: allow any origin (mirrors the CORS allow-all policy).
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Frame is the envelope for every server -> client message.
type Frame struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

type client struct {
	userID int64
	conn   *websocket.Conn
	send   chan []byte
}

// Hub tracks connections keyed by user id. A user may have several
// simultaneous connections (multiple tabs); every one gets each update.
type Hub struct {
	mu      sync.RWMutex
	clients map[int64]map[*client]struct{}
}

// NewHub creates an empty hub.
func NewHub() *Hub {
	return &Hub{clients: make(map[int64]map[*client]struct{})}
}

func (h *Hub) register(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	set, ok := h.clients[c.userID]
	if !ok {
		set = make(map[*client]struct{})
		h.clients[c.userID] = set
	}
	set[c] = struct{}{}
}

func (h *Hub) unregister(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set, ok := h.clients[c.userID]; ok {
		if _, present := set[c]; present {
			delete(set, c)
			close(c.send)
			if len(set) == 0 {
				delete(h.clients, c.userID)
			}
		}
	}
}

// SendToUser pushes a typed frame to every open connection of a user.
// Slow connections that cannot keep up are skipped (never block the caller).
func (h *Hub) SendToUser(userID int64, frameType string, payload any) {
	data, err := json.Marshal(Frame{Type: frameType, Payload: payload})
	if err != nil {
		slog.Error("ws marshal frame", "error", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients[userID] {
		select {
		case c.send <- data:
		default:
			slog.Warn("ws client send buffer full, dropping frame", "user_id", userID)
		}
	}
}

// ServeWS upgrades the HTTP request to a websocket for an already
// authenticated user and starts the read/write pumps.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, userID int64) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("ws upgrade failed", "error", err)
		return
	}

	c := &client{userID: userID, conn: conn, send: make(chan []byte, sendBuffer)}
	h.register(c)

	go c.writePump()
	go c.readPump(h)
}

// readPump discards client messages but keeps the pong deadline fresh and
// triggers a clean unregister when the connection dies.
func (c *client) readPump(h *Hub) {
	defer func() {
		h.unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(4096)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

// writePump serializes all writes to the connection and sends keepalive pings.
func (c *client) writePump() {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
