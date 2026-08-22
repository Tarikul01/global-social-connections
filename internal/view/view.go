package view

import (
	"html/template"
	"net/http"

	"github.com/findneighbour/findneighbour/internal/auth"
	"github.com/findneighbour/findneighbour/internal/config"
	"github.com/gin-gonic/gin"
)

func Render(c *gin.Context, tmpl *template.Template, cfg *config.Config, name string, data gin.H) {
	if data == nil {
		data = gin.H{}
	}
	data["AppURL"] = cfg.AppURL
	data["SupabaseURL"] = cfg.SupabaseURL
	if t, ok := c.Get("csrf_token"); ok {
		data["CSRFToken"] = t
	}
	if u, ok := auth.CurrentUser(c); ok {
		data["CurrentUser"] = u
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, name, data); err != nil {
		c.String(http.StatusInternalServerError, "Page failed to render")
	}
}
