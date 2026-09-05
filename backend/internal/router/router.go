package router

import (
	"html/template"
	"net/http"

	"github.com/findneighbour/findneighbour/internal/auth"
	"github.com/findneighbour/findneighbour/internal/chat"
	"github.com/findneighbour/findneighbour/internal/config"
	"github.com/findneighbour/findneighbour/internal/connection"
	"github.com/findneighbour/findneighbour/internal/controllers"
	"github.com/findneighbour/findneighbour/internal/database"
	"github.com/findneighbour/findneighbour/internal/discovery"
	"github.com/findneighbour/findneighbour/internal/event"
	"github.com/findneighbour/findneighbour/internal/follow"
	"github.com/findneighbour/findneighbour/internal/location"
	"github.com/findneighbour/findneighbour/internal/logger"
	"github.com/findneighbour/findneighbour/internal/middleware"
	"github.com/findneighbour/findneighbour/internal/notification"
	"github.com/findneighbour/findneighbour/internal/profile"
	"github.com/findneighbour/findneighbour/internal/realtime"
	"github.com/findneighbour/findneighbour/internal/social"
	"github.com/findneighbour/findneighbour/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Deps holds everything SetupRouter needs for the JSON API.
type Deps struct {
	Config *config.Config
	DB     *database.DB
	Pool   *pgxpool.Pool
	Log    *logger.Logger
	Tmpl   *template.Template // unused (API-only); kept for controller constructors

	AuthSvc *auth.Service

	UserCtrl    *controllers.UserController
	AuthCtrl    *auth.Controller
	ProfileCtrl *profile.Controller
	FollowCtrl  *follow.Controller
	ConnCtrl    *connection.Controller
	DiscCtrl    *discovery.Controller
	LocCtrl     *location.Controller
	EventCtrl   *event.Controller
	ChatCtrl    *chat.Controller
	NotifCtrl   *notification.Controller
	NotifSvc    *notification.Service
	SocialCtrl  *social.Controller
	WSHandler   *realtime.Handler
	StorageSvc  *storage.Service
}

// SetupRouter builds a Gin engine that serves JSON REST only under /api/v1.
// HTML / HTMX views have been removed — the React SPA is the frontend.
func SetupRouter(d Deps) *gin.Engine {
	if d.Config.IsProduction {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	_ = r.SetTrustedProxies([]string{"0.0.0.0/0", "::/0"})
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger(d.Log))
	r.Use(middleware.SecurityHeaders(d.Config.IsProduction))

	// Optional session on all requests (does not require login).
	r.Use(middleware.Auth(d.AuthSvc, d.Config))
	if d.NotifSvc != nil {
		notifSvc := d.NotifSvc
		r.Use(middleware.UnreadNotifications(notifSvc.UnreadCount))
	}

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Convenience alias so older clients still work.
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"name":    "FindNeighbour API",
				"version": "v1",
				"docs":    "/api/v1/health",
			},
		})
	})

	registerAPIv1(r, d)
	return r
}

func registerAPIv1(r *gin.Engine, d Deps) {
	api := r.Group("/api/v1")
	api.Use(middleware.CORS())
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "ok"}})
		})

		// Auth (public)
		api.POST("/auth/login", middleware.RateLimit(d.Pool, "login", 5), d.AuthCtrl.LoginJSON)
		api.POST("/auth/register", middleware.RateLimit(d.Pool, "register", 3), d.AuthCtrl.RegisterJSON)
		api.POST("/auth/logout", d.AuthCtrl.LogoutJSON)
		api.GET("/auth/me", d.AuthCtrl.MeJSON)
		api.GET("/auth/csrf", d.AuthCtrl.CSRFJSON)

		// Public discovery
		api.GET("/users/nearby", d.UserCtrl.GetNearbyJSON)
		api.GET("/users/:id", d.UserCtrl.GetByIDJSON)
		api.GET("/professions", d.UserCtrl.ListProfessionsJSON)
		api.GET("/stats", d.UserCtrl.StatsJSON)
		api.GET("/profiles/:username", d.ProfileCtrl.PublicProfileJSON)
		api.GET("/events/:id", d.EventCtrl.ShowJSON)
		if d.StorageSvc != nil {
			api.GET("/media/*filepath", d.StorageSvc.ServeMedia)
		}

		// Realtime map (WebSocket) — replaces HTTP long-polling
		if d.WSHandler != nil {
			api.GET("/ws", d.WSHandler.ServeWS)
		}

		// Authenticated API (JSON 401 + CSRF on mutating methods)
		protected := api.Group("/")
		protected.Use(middleware.RequireAuthAPI())
		protected.Use(middleware.CSRF(d.Config))
		{
			protected.GET("/me", d.AuthCtrl.MeJSON)
			protected.GET("/me/profile", d.ProfileCtrl.MeProfileJSON)
			protected.POST("/me/photo", d.ProfileCtrl.UploadPhotoJSON)
			protected.GET("/me/onboarding", d.ProfileCtrl.OnboardingJSON)
			protected.POST("/me/onboarding", d.ProfileCtrl.CompleteOnboardingJSON)
			protected.POST("/me/location", d.LocCtrl.UpdateLocationJSON)
			protected.POST("/live-location", d.LocCtrl.UpdateLiveLocation)
			protected.POST("/live-location/start", d.LocCtrl.StartLiveLocation)
			protected.GET("/live-location/status", d.LocCtrl.LiveStatus)
			protected.DELETE("/live-location", d.LocCtrl.StopLiveLocation)

			protected.GET("/events", d.EventCtrl.ListJSON)
			protected.POST("/events", d.EventCtrl.CreateJSON)
			protected.GET("/events/hosted", d.EventCtrl.ListHostedJSON)
			// GET /events/:id is public (share links) — see open API group above
			protected.POST("/events/:id/join", d.EventCtrl.JoinJSON)
			protected.POST("/events/:id/invite", d.EventCtrl.InviteJSON)
			protected.POST("/events/:id/invitations/:invId/accept", d.EventCtrl.AcceptInvitationJSON)
			protected.POST("/events/:id/invitations/:invId/decline", d.EventCtrl.DeclineInvitationJSON)
			protected.POST("/events/:id/requests/:reqId/accept", d.EventCtrl.AcceptJoinJSON)
			protected.POST("/events/:id/requests/:reqId/reject", d.EventCtrl.RejectJoinJSON)
			protected.GET("/events/:id/map-members", d.EventCtrl.EventMapJSON)

			protected.GET("/notifications", d.NotifCtrl.ListJSON)
			protected.POST("/notifications/:id/read", d.NotifCtrl.MarkReadJSON)

			protected.GET("/connections", d.ConnCtrl.ListJSON)
			protected.GET("/connections/requests", d.ConnCtrl.PendingRequestsJSON)

			protected.GET("/conversations", d.ChatCtrl.ListJSON)
			protected.GET("/conversations/:id/messages", d.ChatCtrl.MessagesAPI)
			protected.POST("/conversations/:id/messages", d.ChatCtrl.SendMessage)
		}
	}
}
