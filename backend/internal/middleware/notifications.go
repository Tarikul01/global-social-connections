package middleware

import (
	"context"

	"github.com/findneighbour/findneighbour/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UnreadCountFunc loads unread notification count for the current user.
type UnreadCountFunc func(ctx context.Context, userID uuid.UUID) (int, error)

// UnreadNotifications attaches unread notification count for the nav badge.
func UnreadNotifications(countFn UnreadCountFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if u, ok := auth.CurrentUser(c); ok && countFn != nil {
			n, err := countFn(c.Request.Context(), u.ID)
			if err == nil {
				c.Set("unread_notifications", n)
			}
		}
		c.Next()
	}
}
