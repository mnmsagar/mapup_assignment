// Package alerts delivers geofence alerts to WebSocket clients.
package alerts

import (
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 50 * time.Second // must be < pongWait
	sendBufferSize = 64
)

// Hub fans out messages to every connected client. All client-set mutations
// happen on the Run goroutine, so no locks are needed.
type Hub struct {
	clients    map[*client]struct{}
	register   chan *client
	unregister chan *client
	broadcast  chan []byte
	count      chan chan int
	upgrader   websocket.Upgrader
}

type client struct {
	conn *websocket.Conn
	send chan []byte
}

func NewHub(allowedOrigins []string) *Hub {
	allowAll := slices.Contains(allowedOrigins, "*")
	return &Hub{
		clients:    map[*client]struct{}{},
		register:   make(chan *client),
		unregister: make(chan *client),
		broadcast:  make(chan []byte, 256),
		count:      make(chan chan int),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 4096,
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				return allowAll || origin == "" || slices.Contains(allowedOrigins, origin)
			},
		},
	}
}

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.clients[c] = struct{}{}
			slog.Info("ws client connected", "clients", len(h.clients))
		case c := <-h.unregister:
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
				slog.Info("ws client disconnected", "clients", len(h.clients))
			}
		case msg := <-h.broadcast:
			for c := range h.clients {
				select {
				case c.send <- msg:
				default: // slow consumer: drop it rather than block everyone
					delete(h.clients, c)
					close(c.send)
					slog.Warn("dropped slow ws client")
				}
			}
		case reply := <-h.count:
			reply <- len(h.clients)
		}
	}
}

// Broadcast queues msg for delivery; it never blocks the caller.
func (h *Hub) Broadcast(msg []byte) {
	select {
	case h.broadcast <- msg:
	default:
		slog.Warn("broadcast queue full, alert dropped from live feed (still stored)")
	}
}

func (h *Hub) Clients() int {
	reply := make(chan int)
	h.count <- reply
	return <-reply
}

// ServeWS upgrades the request and starts the client's pumps.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("ws upgrade failed", "err", err)
		return
	}
	c := &client{conn: conn, send: make(chan []byte, sendBufferSize)}
	h.register <- c
	go c.writePump()
	go c.readPump(h)
}

// readPump discards inbound messages but is required to process pong/close frames.
func (c *client) readPump(h *Hub) {
	defer func() {
		h.unregister <- c
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

func (c *client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, nil)
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
