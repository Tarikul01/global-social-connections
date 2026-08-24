// Package bootstrap exposes app startup for the Vercel Go handler.
// Vercel compiles api/ outside this module, so it cannot import internal/ packages
// directly; this public package bridges that gap.
package bootstrap

import (
	"context"
	"net/http"
	"sync"

	"github.com/findneighbour/findneighbour/internal/app"
	"github.com/findneighbour/findneighbour/internal/config"
)

var (
	once   sync.Once
	router http.Handler
	initErr error
)

// Handler returns the shared Gin engine (HTML + /api/v1) for serverless use.
func Handler() (http.Handler, error) {
	once.Do(func() {
		cfg, err := config.Load()
		if err != nil {
			initErr = err
			return
		}
		application, err := app.New(context.Background(), cfg)
		if err != nil {
			initErr = err
			return
		}
		router = application.Router
	})
	return router, initErr
}
