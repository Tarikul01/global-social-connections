package notification

import (
	"html/template"
	"net/http"

	"github.com/findneighbour/findneighbour/internal/auth"
	"github.com/findneighbour/findneighbour/internal/config"
	"github.com/findneighbour/findneighbour/internal/logger"
	"github.com/findneighbour/findneighbour/internal/view"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Controller struct {
	svc  *Service
	auth *auth.Service
	cfg  *config.Config
	log  *logger.Logger
	tmpl *template.Template
}

func NewController(svc *Service, auth *auth.Service, log *logger.Logger, tmpl *template.Template) *Controller {
	return &Controller{svc: svc, auth: auth, log: log, tmpl: tmpl}
}

func NewControllerWithConfig(svc *Service, auth *auth.Service, cfg *config.Config, log *logger.Logger, tmpl *template.Template) *Controller {
	return &Controller{svc: svc, auth: auth, cfg: cfg, log: log, tmpl: tmpl}
}

func (ctrl *Controller) List(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	list, _ := ctrl.svc.List(c.Request.Context(), u.ID)
	view.Render(c, ctrl.tmpl, ctrl.cfg, "notifications/list.html", gin.H{"Title": "Notifications", "Notifications": list})
}

func (ctrl *Controller) MarkRead(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}
	id, _ := uuid.Parse(c.Param("id"))
	_ = ctrl.svc.MarkRead(c.Request.Context(), id, u.ID)
	c.Redirect(http.StatusFound, "/notifications")
}
