package handler

import (
	"context"
	"net/http"
	"sync"

	"github.com/findneighbour/findneighbour/internal/app"
	"github.com/findneighbour/findneighbour/internal/config"
)

var (
	once        sync.Once
	application *app.App
	initErr     error
)

func getApp() (*app.App, error) {
	once.Do(func() {
		cfg, err := config.Load()
		if err != nil {
			initErr = err
			return
		}
		application, initErr = app.New(context.Background(), cfg)
	})
	return application, initErr
}

// Handler is the single Vercel serverless entrypoint.
// All traffic (HTML UI and /api/v1 JSON) is handled by one Gin engine
// built via router.SetupRouter inside app.New.
func Handler(w http.ResponseWriter, r *http.Request) {
	a, err := getApp()
	if err != nil {
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}
	a.Router.ServeHTTP(w, r)
}
