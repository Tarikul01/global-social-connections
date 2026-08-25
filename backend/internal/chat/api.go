package chat

import (
	"net/http"

	"github.com/findneighbour/findneighbour/internal/auth"
	"github.com/gin-gonic/gin"
)

// ListJSON serves GET /api/v1/conversations
func (ctrl *Controller) ListJSON(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}
	convs, _ := ctrl.svc.ListConversations(c.Request.Context(), u.ID)
	if convs == nil {
		convs = []Conversation{}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": convs})
}
