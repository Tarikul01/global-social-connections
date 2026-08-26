package realtime

import (
	"net/http"

	"github.com/findneighbour/findneighbour/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Cookie session + CORS-facing SPA; allow any origin in dev.
		// Production should still rely on SameSite cookies.
		return true
	},
}

// Handler upgrades HTTP to WebSocket and registers the client on the hub.
type Handler struct {
	hub *Hub
}

func NewHandler(hub *Hub) *Handler {
	return &Handler{hub: hub}
}

func (h *Handler) ServeWS(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	viewerID := uuid.Nil
	if u, ok := auth.CurrentUser(c); ok {
		viewerID = u.ID
	}

	client := &Client{
		hub:      h.hub,
		conn:     conn,
		send:     make(chan []byte, 16),
		viewerID: viewerID,
	}
	h.hub.register <- client

	go client.writePump()
	client.readPump()
}

// DebouncedBroadcaster notifies the hub that the map should refresh.
// Hub.BroadcastMapChanged already debounces; this keeps the old interface
// for HTTP controllers (login/logout, legacy live-location routes).
type DebouncedBroadcaster struct {
	hub *Hub
}

func NewDebouncedBroadcaster(hub *Hub) *DebouncedBroadcaster {
	return &DebouncedBroadcaster{hub: hub}
}

func (d *DebouncedBroadcaster) BroadcastMapChanged() {
	d.hub.BroadcastMapChanged()
}
