package realtime

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Envelope is the wire format for all WebSocket messages.
type Envelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// SubscribeData is sent by the client when the map viewport/filters change.
type SubscribeData struct {
	Lat         float64  `json:"lat"`
	Lng         float64  `json:"lng"`
	RadiusKm    float64  `json:"radius_km"`
	Status      string   `json:"status"`
	Query       string   `json:"q"`
	Professions []string `json:"professions"`
}

// NearbyProvider loads a map snapshot for a subscriber.
type NearbyProvider interface {
	NearbySnapshot(viewerID uuid.UUID, sub SubscribeData) (any, error)
}

// Presence reports who is currently connected to the map (realtime online).
type Presence interface {
	IsOnline(userID uuid.UUID) bool
	OnlineCount() int
	OnlineIDs() []uuid.UUID
}

// Client is one browser WebSocket connection.
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	viewerID uuid.UUID
	mu       sync.Mutex
	sub      *SubscribeData
}

// Hub tracks live map clients, authenticated presence, and map pushes.
type Hub struct {
	mu         sync.RWMutex
	clients    map[*Client]struct{}
	presence   map[uuid.UUID]int // ref-count tabs/devices per user
	register   chan *Client
	unregister chan *Client
	provider   NearbyProvider
}

func NewHub(provider NearbyProvider) *Hub {
	return &Hub{
		clients:    make(map[*Client]struct{}),
		presence:   make(map[uuid.UUID]int),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		provider:   provider,
	}
}

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			changed := false
			h.mu.Lock()
			h.clients[c] = struct{}{}
			if c.viewerID != uuid.Nil {
				h.presence[c.viewerID]++
				if h.presence[c.viewerID] == 1 {
					changed = true
				}
			}
			h.mu.Unlock()
			if changed {
				h.BroadcastMapChanged()
			}

		case c := <-h.unregister:
			changed := false
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
			if c.viewerID != uuid.Nil {
				if n := h.presence[c.viewerID]; n <= 1 {
					delete(h.presence, c.viewerID)
					changed = true
				} else {
					h.presence[c.viewerID] = n - 1
				}
			}
			h.mu.Unlock()
			if changed {
				h.BroadcastMapChanged()
			}
		}
	}
}

func (h *Hub) IsOnline(userID uuid.UUID) bool {
	if userID == uuid.Nil {
		return false
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.presence[userID] > 0
}

func (h *Hub) OnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.presence)
}

func (h *Hub) OnlineIDs() []uuid.UUID {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]uuid.UUID, 0, len(h.presence))
	for id := range h.presence {
		out = append(out, id)
	}
	return out
}

// BroadcastMapChanged re-pushes nearby snapshots to every subscribed client.
func (h *Hub) BroadcastMapChanged() {
	go h.RefreshAll()
}

func (h *Hub) setSubscription(c *Client, sub SubscribeData) {
	c.mu.Lock()
	cp := sub
	c.sub = &cp
	c.mu.Unlock()
	h.pushNearby(c)
}

func (h *Hub) pushNearby(c *Client) {
	c.mu.Lock()
	sub := c.sub
	viewer := c.viewerID
	c.mu.Unlock()
	if sub == nil || h.provider == nil {
		return
	}
	data, err := h.provider.NearbySnapshot(viewer, *sub)
	if err != nil {
		errBody, _ := json.Marshal(Envelope{
			Type: "error",
			Data: mustJSON(map[string]string{"message": "failed to load nearby users"}),
		})
		select {
		case c.send <- errBody:
		default:
		}
		return
	}
	body, _ := json.Marshal(Envelope{Type: "nearby", Data: mustJSON(data)})
	select {
	case c.send <- body:
	default:
	}
}

// RefreshAll re-pushes nearby snapshots to every subscribed client.
func (h *Hub) RefreshAll() {
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()
	for _, c := range clients {
		h.pushNearby(c)
	}
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 50 * time.Second
	maxMsgSize = 32 * 1024
)

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMsgSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		var env Envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			continue
		}
		switch env.Type {
		case "subscribe":
			var sub SubscribeData
			if err := json.Unmarshal(env.Data, &sub); err != nil {
				continue
			}
			if sub.RadiusKm <= 0 {
				sub.RadiusKm = 40
			}
			c.hub.setSubscription(c, sub)
		case "ping":
			pong, _ := json.Marshal(Envelope{Type: "pong"})
			select {
			case c.send <- pong:
			default:
			}
		case "refresh":
			c.hub.pushNearby(c)
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
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
