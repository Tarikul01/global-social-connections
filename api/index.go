package handler

import (
	"net/http"

	"github.com/findneighbour/findneighbour/bootstrap"
)

// Handler is the single Vercel serverless entrypoint.
// All traffic (HTML UI and /api/v1 JSON) is handled by one Gin engine.
func Handler(w http.ResponseWriter, r *http.Request) {
	h, err := bootstrap.Handler()
	if err != nil {
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}
	h.ServeHTTP(w, r)
}
