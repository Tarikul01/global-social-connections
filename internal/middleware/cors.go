package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS allows mobile clients (Flutter / React Native) and other origins
// to call the JSON API. Apply only to the /api/v1 route group.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			origin = "*"
		}
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Request-ID, X-CSRF-Token")
		c.Header("Access-Control-Expose-Headers", "X-Request-ID")
		c.Header("Access-Control-Max-Age", "86400")
		if origin != "*" {
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Vary", "Origin")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
