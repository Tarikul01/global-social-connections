package notification

import (
	"net/http"

	"github.com/findneighbour/findneighbour/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListJSON serves GET /api/v1/notifications
func (ctrl *Controller) ListJSON(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}
	list, _ := ctrl.svc.List(c.Request.Context(), u.ID)
	items := make([]Item, 0, len(list))
	for _, n := range list {
		item := Item{
			Notification: n,
			Unread:       n.ReadAt == nil,
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
		items = append(items, item)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
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
