package connection

import (
	"net/http"

	"github.com/findneighbour/findneighbour/internal/auth"
	"github.com/gin-gonic/gin"
)

// ListJSON serves GET /api/v1/connections
func (ctrl *Controller) ListJSON(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}
	list, _ := ctrl.svc.List(c.Request.Context(), u.ID)
	if list == nil {
		list = []Connection{}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}

// PendingRequestsJSON serves GET /api/v1/connections/requests
func (ctrl *Controller) PendingRequestsJSON(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}
	list, _ := ctrl.svc.Pending(c.Request.Context(), u.ID)
	if list == nil {
		list = []Connection{}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}
