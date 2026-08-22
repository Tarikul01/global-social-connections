package app

import (
	"context"
	"net/http"

	"github.com/findneighbour/findneighbour/internal/auth"
	"github.com/findneighbour/findneighbour/internal/chat"
	"github.com/findneighbour/findneighbour/internal/config"
	"github.com/findneighbour/findneighbour/internal/connection"
	"github.com/findneighbour/findneighbour/internal/database"
	"github.com/findneighbour/findneighbour/internal/discovery"
	"github.com/findneighbour/findneighbour/internal/event"
	"github.com/findneighbour/findneighbour/internal/follow"
	"github.com/findneighbour/findneighbour/internal/location"
	"github.com/findneighbour/findneighbour/internal/logger"
	"github.com/findneighbour/findneighbour/internal/middleware"
	"github.com/findneighbour/findneighbour/internal/notification"
	"github.com/findneighbour/findneighbour/internal/profile"
	"github.com/findneighbour/findneighbour/internal/security"
	"github.com/findneighbour/findneighbour/internal/social"
	"github.com/findneighbour/findneighbour/internal/storage"
	"github.com/findneighbour/findneighbour/web"
	"github.com/gin-gonic/gin"
	"html/template"
)

type App struct {
	Config *config.Config
	DB     *database.DB
	Log    *logger.Logger
	Router *gin.Engine
	Tmpl   *template.Template
}

func New(ctx context.Context, cfg *config.Config) (*App, error) {
	if cfg.IsProduction {
		gin.SetMode(gin.ReleaseMode)
	}

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	log := logger.New(cfg.IsProduction)
	tmpl, err := web.LoadTemplates()
	if err != nil {
		db.Close()
		return nil, err
	}

	argonParams := security.ArgonParams{
		Memory: cfg.ArgonMemory, Iterations: cfg.ArgonIter,
		Parallelism: cfg.ArgonParallel, SaltLength: 16, KeyLength: 32,
	}

	authRepo := auth.NewRepository(db.Pool)
	authSvc := auth.NewService(authRepo, cfg, argonParams, log)

	profileRepo := profile.NewRepository(db.Pool)
	profileSvc := profile.NewService(profileRepo, log)
	storageSvc := storage.NewService(cfg, db.Pool, log)

	followRepo := follow.NewRepository(db.Pool)
	followSvc := follow.NewService(followRepo, db.Pool, log)

	connRepo := connection.NewRepository(db.Pool)
	connSvc := connection.NewService(connRepo, db.Pool, log)
	connSvc.SetDB(db)

	locRepo := location.NewRepository(db.Pool)
	locSvc := location.NewService(locRepo, log)

	discSvc := discovery.NewService(db.Pool, log)

	eventRepo := event.NewRepository(db.Pool)
	eventRepo.SetDB(db)
	eventSvc := event.NewService(eventRepo, db.Pool, log)

	chatRepo := chat.NewRepository(db.Pool)
	chatSvc := chat.NewService(chatRepo, db.Pool, log)

	notifRepo := notification.NewRepository(db.Pool)
	notifSvc := notification.NewService(notifRepo, log)

	socialSvc := social.NewService(db.Pool, log)

	authCtrl := auth.NewController(authSvc, cfg, log, tmpl)
	profileCtrl := profile.NewController(profileSvc, storageSvc, authSvc, cfg, log, tmpl)
	followCtrl := follow.NewController(followSvc, authSvc, log, tmpl)
	connCtrl := connection.NewControllerWithConfig(connSvc, authSvc, notifSvc, cfg, log, tmpl)
	discCtrl := discovery.NewController(discSvc, locSvc, authSvc, cfg, log, tmpl)
	locCtrl := location.NewControllerWithConfig(locSvc, authSvc, cfg, log, tmpl)
	eventCtrl := event.NewController(eventSvc, authSvc, notifSvc, cfg, log, tmpl)
	eventCtrl.SetLocationService(locSvc)
	chatCtrl := chat.NewController(chatSvc, authSvc, notifSvc, cfg, log, tmpl)
	notifCtrl := notification.NewControllerWithConfig(notifSvc, authSvc, cfg, log, tmpl)
	socialCtrl := social.NewController(socialSvc, authSvc, log, tmpl)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger(log))
	r.Use(middleware.SecurityHeaders(cfg.IsProduction))
	r.Any("/static/*filepath", gin.WrapH(http.StripPrefix("/static", web.StaticHandler())))

	// Optional session on all pages (does not require login)
	r.Use(middleware.Auth(authSvc, cfg))

	r.GET("/", discCtrl.LandingMap)
	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	r.GET("/api/map/people", discCtrl.PublicMapAPI)

	r.GET("/auth/login", authCtrl.LoginForm)
	r.POST("/auth/login", middleware.RateLimit(db.Pool, "login", 5), authCtrl.Login)
	r.GET("/auth/register", authCtrl.RegisterForm)
	r.POST("/auth/register", middleware.RateLimit(db.Pool, "register", 3), authCtrl.Register)
	r.POST("/auth/logout", authCtrl.Logout)
	r.GET("/auth/verify", authCtrl.VerifyEmail)

	authed := r.Group("/")
	authed.Use(middleware.RequireAuth())
	authed.Use(middleware.CSRF(cfg))
	{
		authed.GET("/dashboard", profileCtrl.Dashboard)
		authed.GET("/profile", profileCtrl.ShowOwn)
		authed.GET("/profile/edit", profileCtrl.EditForm)
		authed.POST("/profile", profileCtrl.Update)
		authed.POST("/profile/photo", profileCtrl.UploadPhoto)
		authed.GET("/onboarding", profileCtrl.OnboardingForm)
		authed.POST("/onboarding", profileCtrl.CompleteOnboarding)
		authed.GET("/users/:username", profileCtrl.ShowPublic)

		authed.POST("/users/:id/follow", followCtrl.Follow)
		authed.DELETE("/users/:id/follow", followCtrl.Unfollow)

		authed.POST("/users/:id/connect", connCtrl.SendRequest)
		authed.GET("/connections", connCtrl.List)
		authed.GET("/connections/requests", connCtrl.PendingRequests)
		authed.POST("/connections/:id/accept", connCtrl.Accept)
		authed.POST("/connections/:id/reject", connCtrl.Reject)

		authed.GET("/discover", discCtrl.Index)
		authed.GET("/api/discover/nearby", discCtrl.NearbyAPI)

		authed.GET("/settings/location", locCtrl.SettingsForm)
		authed.POST("/settings/location", locCtrl.UpdateSettings)
		authed.POST("/settings/location/update", locCtrl.UpdateLocation)
		authed.POST("/api/live-location", locCtrl.UpdateLiveLocation)
		authed.DELETE("/api/live-location", locCtrl.StopLiveLocation)

		authed.GET("/events", eventCtrl.List)
		authed.GET("/events/new", eventCtrl.CreateForm)
		authed.POST("/events", eventCtrl.Create)
		authed.GET("/events/:id", eventCtrl.Show)
		authed.POST("/events/:id/join", eventCtrl.JoinRequest)
		authed.POST("/events/:id/requests/:reqId/accept", eventCtrl.AcceptJoin)
		authed.POST("/events/:id/requests/:reqId/reject", eventCtrl.RejectJoin)
		authed.GET("/events/:id/map", eventCtrl.MapView)
		authed.POST("/events/:id/live-consent", eventCtrl.ToggleLiveConsent)

		authed.GET("/messages", chatCtrl.List)
		authed.GET("/messages/:id", chatCtrl.Show)
		authed.GET("/api/conversations/:id/messages", chatCtrl.MessagesAPI)
		authed.POST("/api/conversations/:id/messages", chatCtrl.SendMessage)

		authed.GET("/notifications", notifCtrl.List)
		authed.POST("/notifications/:id/read", notifCtrl.MarkRead)

		authed.POST("/users/:id/block", socialCtrl.Block)
		authed.DELETE("/users/:id/block", socialCtrl.Unblock)
		authed.POST("/reports", socialCtrl.Report)
	}

	return &App{Config: cfg, DB: db, Log: log, Router: r, Tmpl: tmpl}, nil
}

func (a *App) Close() {
	a.DB.Close()
}
