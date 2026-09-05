package notification

import (
	"net/http"
	"time"

	"github.com/findneighbour/findneighbour/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type listItemJSON struct {
	ID            string `json:"id"`
	Type          string `json:"type"`
	Title         string `json:"title"`
	Body          string `json:"body"`
	Unread        bool   `json:"unread"`
	InvitePending bool   `json:"invite_pending"`
	EventID       string `json:"event_id,omitempty"`
	InvitationID  string `json:"invitation_id,omitempty"`
	CreatedAt     string `json:"created_at"`
}

func (ctrl *Controller) toListItem(c *gin.Context, u *auth.User, n Notification) listItemJSON {
	item := listItemJSON{
		ID:        n.ID.String(),
		Type:      n.Type,
		Title:     n.Title,
		Body:      n.Body,
		Unread:    n.ReadAt == nil,
		CreatedAt: n.CreatedAt.UTC().Format(time.RFC3339),
	}
	if n.Data != nil {
		item.EventID = n.Data["event_id"]
		item.InvitationID = n.Data["invitation_id"]
	}
	if n.Type == "event_invitation" && item.InvitationID != "" && ctrl.inviteStatus != nil {
		invID, err := uuid.Parse(item.InvitationID)
		if err == nil {
			status, err := ctrl.inviteStatus(c.Request.Context(), invID, u.ID)
			if err == nil && status == "pending" {
				item.InvitePending = true
			}
		}
	}
	return item
}

// ListJSON serves GET /api/v1/notifications
func (ctrl *Controller) ListJSON(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}
	list, _ := ctrl.svc.List(c.Request.Context(), u.ID)
	items := make([]listItemJSON, 0, len(list))
	unread := 0
	for _, n := range list {
		item := ctrl.toListItem(c, u, n)
		if item.Unread {
			unread++
		}
		items = append(items, item)
	}
	// Prefer authoritative unread count (not capped by list limit).
	if n, err := ctrl.svc.UnreadCount(c.Request.Context(), u.ID); err == nil {
		unread = n
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    items,
		"meta":    gin.H{"unread": unread},
	})
}

// UnreadCountJSON serves GET /api/v1/notifications/unread-count
func (ctrl *Controller) UnreadCountJSON(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}
	n, err := ctrl.svc.UnreadCount(c.Request.Context(), u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"unread": n}})
}

// MarkReadJSON serves POST /api/v1/notifications/:id/read
func (ctrl *Controller) MarkReadJSON(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid id"})
		return
	}
	_ = ctrl.svc.MarkRead(c.Request.Context(), id, u.ID)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"read": true}})
}
