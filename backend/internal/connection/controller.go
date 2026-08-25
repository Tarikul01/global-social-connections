package connection

import (
	"html/template"
	"net/http"

	"github.com/findneighbour/findneighbour/internal/auth"
	"github.com/findneighbour/findneighbour/internal/config"
	"github.com/findneighbour/findneighbour/internal/logger"
	"github.com/findneighbour/findneighbour/internal/notification"
	"github.com/findneighbour/findneighbour/internal/view"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Controller struct {
	svc    *Service
	auth   *auth.Service
	notif  *notification.Service
	cfg    *config.Config
	log    *logger.Logger
	tmpl   *template.Template
}

func NewController(svc *Service, auth *auth.Service, notif *notification.Service, log *logger.Logger, tmpl *template.Template) *Controller {
	return &Controller{svc: svc, auth: auth, notif: notif, log: log, tmpl: tmpl}
}

func NewControllerWithConfig(svc *Service, auth *auth.Service, notif *notification.Service, cfg *config.Config, log *logger.Logger, tmpl *template.Template) *Controller {
	return &Controller{svc: svc, auth: auth, notif: notif, cfg: cfg, log: log, tmpl: tmpl}
}

func (ctrl *Controller) SendRequest(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	targetID, _ := uuid.Parse(c.Param("id"))
	conn, err := ctrl.svc.SendRequest(c.Request.Context(), u.ID, targetID, c.PostForm("message"))
	if err != nil {
		c.String(422, err.Error())
		return
	}
	_ = ctrl.notif.Create(c.Request.Context(), targetID, "connection_request", "New connection request", "Someone wants to connect with you", map[string]string{"connection_id": conn.ID.String()})
	c.Redirect(http.StatusFound, "/connections/requests")
}

func (ctrl *Controller) List(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	list, _ := ctrl.svc.List(c.Request.Context(), u.ID)
	view.Render(c, ctrl.tmpl, ctrl.cfg, "connections/list.html", gin.H{"Title": "Connections", "Connections": list})
}

func (ctrl *Controller) PendingRequests(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	list, _ := ctrl.svc.Pending(c.Request.Context(), u.ID)
	view.Render(c, ctrl.tmpl, ctrl.cfg, "connections/requests.html", gin.H{"Title": "Connection Requests", "Requests": list})
}

func (ctrl *Controller) Accept(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	connID, _ := uuid.Parse(c.Param("id"))
	conn, _ := ctrl.svc.GetByID(c.Request.Context(), connID)
	if err := ctrl.svc.Accept(c.Request.Context(), connID, u.ID); err != nil {
		c.String(422, err.Error())
		return
	}
	if conn != nil {
		_ = ctrl.notif.Create(c.Request.Context(), conn.RequesterID, "connection_accepted", "Connection accepted", "Your connection request was accepted", nil)
	}
	c.Redirect(http.StatusFound, "/connections")
}

func (ctrl *Controller) Reject(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	connID, _ := uuid.Parse(c.Param("id"))
	_ = ctrl.svc.Reject(c.Request.Context(), connID, u.ID)
	c.Redirect(http.StatusFound, "/connections/requests")
}
