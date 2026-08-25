package middleware

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/findneighbour/findneighbour/internal/auth"
	"github.com/findneighbour/findneighbour/internal/config"
	"github.com/findneighbour/findneighbour/internal/security"
	"github.com/findneighbour/findneighbour/internal/session"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type sessionKey struct{}

func Auth(svc *auth.Service, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(session.CookieName)
		if err != nil || token == "" {
			c.Next()
			return
		}
		user, sess, err := svc.ValidateSession(c.Request.Context(), token)
		if err != nil {
			c.Next()
			return
		}
		c.Set("user", user)
		c.Set("session", sess)
		c.Set("user_id", user.ID.String())
		c.Set("csrf_token", security.GenerateCSRFToken(cfg.CSRFSecret, sess.ID.String()))
		c.Next()
	}
}

func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := c.Get("user"); !ok {
			next := url.QueryEscape(c.Request.URL.RequestURI())
			c.Redirect(http.StatusFound, "/auth/login?next="+next)
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireAuthAPI returns JSON 401 instead of an HTML redirect (for /api/v1).
func RequireAuthAPI() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := c.Get("user"); !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "authentication required",
			})
			return
		}
		c.Next()
	}
}

func CSRF(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		sess, ok := c.Get("session")
		if !ok {
			c.Next()
			return
		}
		s := sess.(*auth.Session)
		token := security.GenerateCSRFToken(cfg.CSRFSecret, s.ID.String())
		c.Set("csrf_token", token)

		if c.Request.Method == "GET" || c.Request.Method == "HEAD" || c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		submitted := c.PostForm("csrf_token")
		if submitted == "" {
			submitted = c.GetHeader("X-CSRF-Token")
		}
		if !security.ValidateCSRFToken(cfg.CSRFSecret, s.ID.String(), submitted) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid csrf token"})
			return
		}
		c.Next()
	}
}

func RateLimit(pool *pgxpool.Pool, endpoint string, max int) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := endpoint + ":" + c.ClientIP()
		ctx := c.Request.Context()
		var count int
		var windowStart time.Time
		err := pool.QueryRow(ctx, `
			INSERT INTO rate_limits (key, count, window_start)
			VALUES ($1, 1, now())
			ON CONFLICT (key) DO UPDATE SET
				count = CASE WHEN rate_limits.window_start < now() - interval '15 minutes'
					THEN 1 ELSE rate_limits.count + 1 END,
				window_start = CASE WHEN rate_limits.window_start < now() - interval '15 minutes'
					THEN now() ELSE rate_limits.window_start END
			RETURNING count, window_start
		`, key).Scan(&count, &windowStart)
		if err != nil {
			c.Next()
			return
		}
		if count > max {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}

func UserFromContext(ctx context.Context) (*auth.User, bool) {
	v := ctx.Value(sessionKey{})
	if v == nil {
		return nil, false
	}
	u, ok := v.(*auth.User)
	return u, ok
}
