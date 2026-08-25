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
	"github.com/findneighbour/findneighbour/internal/social"
	"github.com/findneighbour/findneighbour/web"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Deps holds everything SetupRouter needs to wire Web MVC + JSON API.
type Deps struct {
	Config *config.Config
	DB     *database.DB
	Pool   *pgxpool.Pool
	Log    *logger.Logger
	Tmpl   *template.Template

	AuthSvc *auth.Service

	UserCtrl   *controllers.UserController
	AuthCtrl   *auth.Controller
	ProfileCtrl *profile.Controller
	FollowCtrl *follow.Controller
	ConnCtrl   *connection.Controller
	DiscCtrl   *discovery.Controller
	LocCtrl    *location.Controller
	EventCtrl  *event.Controller
	ChatCtrl   *chat.Controller
	NotifCtrl  *notification.Controller
	NotifSvc   *notification.Service
	SocialCtrl *social.Controller
}

// SetupRouter builds a single Gin engine that serves:
//   - Web UI / HTMX under "/"
//   - Mobile JSON REST under "/api/v1" (with CORS)
func SetupRouter(d Deps) *gin.Engine {
	if d.Config.IsProduction {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	// Vercel / reverse proxies: trust X-Forwarded-For so ClientIP is populated.
	_ = r.SetTrustedProxies([]string{"0.0.0.0/0", "::/0"})
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger(d.Log))
	r.Use(middleware.SecurityHeaders(d.Config.IsProduction))
	r.Any("/static/*filepath", gin.WrapH(http.StripPrefix("/static", web.StaticHandler())))

	// Optional session on all requests (does not require login).
	r.Use(middleware.Auth(d.AuthSvc, d.Config))
	if d.NotifSvc != nil {
		notifSvc := d.NotifSvc
		r.Use(middleware.UnreadNotifications(notifSvc.UnreadCount))
	}

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	registerWeb(r, d)
	registerAPIv1(r, d)

	return r
}

// registerWeb mounts HTML / HTMX routes (no CORS).
func registerWeb(r *gin.Engine, d Deps) {
	webGroup := r.Group("/")
	{
		webGroup.GET("/", d.DiscCtrl.LandingMap)
		webGroup.GET("/explore", d.DiscCtrl.LandingMap)
		webGroup.GET("/discover", d.DiscCtrl.LandingMap)

		webGroup.GET("/auth/login", d.AuthCtrl.LoginForm)
		webGroup.POST("/auth/login", middleware.RateLimit(d.Pool, "login", 5), d.AuthCtrl.Login)
		webGroup.GET("/auth/register", d.AuthCtrl.RegisterForm)
		webGroup.POST("/auth/register", middleware.RateLimit(d.Pool, "register", 3), d.AuthCtrl.Register)
		webGroup.POST("/auth/logout", d.AuthCtrl.Logout)
		webGroup.GET("/auth/verify", d.AuthCtrl.VerifyEmail)

		// Legacy browser JSON used by map JS (same service as /api/v1).
		webGroup.GET("/api/map/people", d.UserCtrl.GetNearbyJSON)
		webGroup.GET("/api/discover/nearby", d.UserCtrl.GetNearbyJSON)
		webGroup.GET("/api/map/professions", d.UserCtrl.ListProfessionsJSON)
	}

	authed := webGroup.Group("/")
	authed.Use(middleware.RequireAuth())
	authed.Use(middleware.CSRF(d.Config))
	{
		authed.GET("/dashboard", d.ProfileCtrl.Dashboard)
		authed.GET("/profile", d.ProfileCtrl.ShowOwn)
		authed.GET("/profile/edit", d.ProfileCtrl.EditForm)
		authed.POST("/profile", d.ProfileCtrl.Update)
		authed.POST("/profile/photo", d.ProfileCtrl.UploadPhoto)
		authed.GET("/onboarding", d.ProfileCtrl.OnboardingForm)
		authed.POST("/onboarding", d.ProfileCtrl.CompleteOnboarding)
		authed.GET("/users/:username", d.ProfileCtrl.ShowPublic)

		authed.POST("/users/:id/follow", d.FollowCtrl.Follow)
		authed.DELETE("/users/:id/follow", d.FollowCtrl.Unfollow)

		authed.POST("/users/:id/connect", d.ConnCtrl.SendRequest)
		authed.GET("/connections", d.ConnCtrl.List)
		authed.GET("/connections/requests", d.ConnCtrl.PendingRequests)
		authed.POST("/connections/:id/accept", d.ConnCtrl.Accept)
		authed.POST("/connections/:id/reject", d.ConnCtrl.Reject)

		authed.GET("/settings/location", d.LocCtrl.SettingsForm)
		authed.POST("/settings/location", d.LocCtrl.UpdateSettings)
		authed.POST("/settings/location/update", d.LocCtrl.UpdateLocation)

		authed.POST("/api/live-location", d.LocCtrl.UpdateLiveLocation)
		authed.POST("/api/live-location/start", d.LocCtrl.StartLiveLocation)
		authed.GET("/api/live-location/status", d.LocCtrl.LiveStatus)
		authed.DELETE("/api/live-location", d.LocCtrl.StopLiveLocation)

		authed.GET("/events", d.EventCtrl.List)
		authed.GET("/events/new", d.EventCtrl.CreateForm)
		authed.POST("/events", d.EventCtrl.Create)
		authed.GET("/api/events/hosted", d.EventCtrl.ListHostedJSON)
		authed.GET("/api/events/:id/map-members", d.EventCtrl.EventMapJSON)
		authed.GET("/events/:id", d.EventCtrl.Show)
		authed.POST("/events/:id/join", d.EventCtrl.JoinRequest)
		authed.POST("/events/:id/invite", d.EventCtrl.Invite)
		authed.POST("/events/:id/invite-batch", d.EventCtrl.InviteBatch)
		authed.GET("/events/:id/users/search", d.EventCtrl.SearchUsers)
		authed.POST("/events/:id/members/:userId/promote-host", d.EventCtrl.PromoteHost)
		authed.POST("/events/:id/requests/:reqId/accept", d.EventCtrl.AcceptJoin)
		authed.POST("/events/:id/requests/:reqId/reject", d.EventCtrl.RejectJoin)
		authed.POST("/events/:id/invitations/:invId/accept", d.EventCtrl.AcceptInvitation)
		authed.POST("/events/:id/invitations/:invId/decline", d.EventCtrl.DeclineInvitation)
		authed.GET("/events/:id/map", d.EventCtrl.MapView)
		authed.POST("/events/:id/live-consent", d.EventCtrl.ToggleLiveConsent)

		authed.GET("/messages", d.ChatCtrl.List)
		authed.GET("/messages/:id", d.ChatCtrl.Show)
		authed.GET("/api/conversations/:id/messages", d.ChatCtrl.MessagesAPI)
		authed.POST("/api/conversations/:id/messages", d.ChatCtrl.SendMessage)

		authed.GET("/notifications", d.NotifCtrl.List)
		authed.POST("/notifications/:id/read", d.NotifCtrl.MarkRead)

		authed.POST("/users/:id/block", d.SocialCtrl.Block)
		authed.DELETE("/users/:id/block", d.SocialCtrl.Unblock)
		authed.POST("/reports", d.SocialCtrl.Report)
	}
}

// registerAPIv1 mounts versioned JSON REST for mobile clients.
func registerAPIv1(r *gin.Engine, d Deps) {
	api := r.Group("/api/v1")
	api.Use(middleware.CORS())
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "ok"}})
		})

		// Public discovery endpoints (optional session enriches results).
		api.GET("/users/nearby", d.UserCtrl.GetNearbyJSON)
		api.GET("/users/:id", d.UserCtrl.GetByIDJSON)
		api.GET("/professions", d.UserCtrl.ListProfessionsJSON)

		// Authenticated API surface (JSON 401, not HTML redirect).
		protected := api.Group("/")
		protected.Use(middleware.RequireAuthAPI())
		{
			protected.GET("/me", func(c *gin.Context) {
				u, _ := auth.CurrentUser(c)
				c.JSON(http.StatusOK, gin.H{
					"success": true,
					"data": gin.H{
						"id":       u.ID,
						"email":    u.Email,
						"username": u.Username,
					},
				})
			})
		}
	}
}
