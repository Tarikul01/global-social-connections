package app

import (
	"context"
	"html/template"

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
	"github.com/findneighbour/findneighbour/internal/notification"
	"github.com/findneighbour/findneighbour/internal/profile"
	"github.com/findneighbour/findneighbour/internal/router"
	"github.com/findneighbour/findneighbour/internal/security"
	"github.com/findneighbour/findneighbour/internal/services"
	"github.com/findneighbour/findneighbour/internal/social"
	"github.com/findneighbour/findneighbour/internal/storage"
	"github.com/findneighbour/findneighbour/web"
	"github.com/gin-gonic/gin"
)

type App struct {
	Config *config.Config
	DB     *database.DB
	Log    *logger.Logger
	Router *gin.Engine
	Tmpl   *template.Template
}

func New(ctx context.Context, cfg *config.Config) (*App, error) {
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
	userSvc := services.NewUserService(discSvc, profileSvc, log)

	eventRepo := event.NewRepository(db.Pool)
	eventRepo.SetDB(db)
	eventSvc := event.NewService(eventRepo, db.Pool, log)

	chatRepo := chat.NewRepository(db.Pool)
	chatSvc := chat.NewService(chatRepo, db.Pool, log)

	notifRepo := notification.NewRepository(db.Pool)
	notifSvc := notification.NewService(notifRepo, log)

	socialSvc := social.NewService(db.Pool, log)

	userCtrl := controllers.NewUserController(userSvc, cfg, log, tmpl)
	authCtrl := auth.NewController(authSvc, cfg, log, tmpl)
	profileCtrl := profile.NewController(profileSvc, storageSvc, authSvc, cfg, log, tmpl)
	profileCtrl.SetLocationService(locSvc)
	followCtrl := follow.NewController(followSvc, authSvc, log, tmpl)
	connCtrl := connection.NewControllerWithConfig(connSvc, authSvc, notifSvc, cfg, log, tmpl)
	discCtrl := discovery.NewController(discSvc, locSvc, authSvc, cfg, log, tmpl)
	locCtrl := location.NewControllerWithConfig(locSvc, authSvc, cfg, log, tmpl)
	eventCtrl := event.NewController(eventSvc, authSvc, notifSvc, cfg, log, tmpl)
	eventCtrl.SetLocationService(locSvc)
	chatCtrl := chat.NewController(chatSvc, authSvc, notifSvc, cfg, log, tmpl)
	notifCtrl := notification.NewControllerWithConfig(notifSvc, authSvc, cfg, log, tmpl)
	socialCtrl := social.NewController(socialSvc, authSvc, log, tmpl)

	r := router.SetupRouter(router.Deps{
		Config:      cfg,
		DB:          db,
		Pool:        db.Pool,
		Log:         log,
		Tmpl:        tmpl,
		AuthSvc:     authSvc,
		UserCtrl:    userCtrl,
		AuthCtrl:    authCtrl,
		ProfileCtrl: profileCtrl,
		FollowCtrl:  followCtrl,
		ConnCtrl:    connCtrl,
		DiscCtrl:    discCtrl,
		LocCtrl:     locCtrl,
		EventCtrl:   eventCtrl,
		ChatCtrl:    chatCtrl,
		NotifCtrl:   notifCtrl,
		SocialCtrl:  socialCtrl,
	})

	return &App{Config: cfg, DB: db, Log: log, Router: r, Tmpl: tmpl}, nil
}

func (a *App) Close() {
	a.DB.Close()
}
