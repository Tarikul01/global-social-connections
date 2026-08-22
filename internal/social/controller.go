package social

import (
	"html/template"
	"net/http"

	"github.com/findneighbour/findneighbour/internal/auth"
	"github.com/findneighbour/findneighbour/internal/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Controller struct {
	svc  *Service
	auth *auth.Service
	log  *logger.Logger
	tmpl *template.Template
}

func NewController(svc *Service, auth *auth.Service, log *logger.Logger, tmpl *template.Template) *Controller {
	return &Controller{svc: svc, auth: auth, log: log, tmpl: tmpl}
}

func (ctrl *Controller) Block(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	targetID, _ := uuid.Parse(c.Param("id"))
	_ = ctrl.svc.Block(c.Request.Context(), u.ID, targetID)
	c.Redirect(http.StatusFound, c.Request.Referer())
}

func (ctrl *Controller) Unblock(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	targetID, _ := uuid.Parse(c.Param("id"))
	_ = ctrl.svc.Unblock(c.Request.Context(), u.ID, targetID)
	c.Redirect(http.StatusFound, c.Request.Referer())
}

func (ctrl *Controller) Report(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	var reportedUser *uuid.UUID
	if id := c.PostForm("reported_user_id"); id != "" {
		uid, _ := uuid.Parse(id)
		reportedUser = &uid
	}
	var reportedEvent *uuid.UUID
	if id := c.PostForm("reported_event_id"); id != "" {
		eid, _ := uuid.Parse(id)
		reportedEvent = &eid
	}
	_ = ctrl.svc.Report(c.Request.Context(), u.ID, c.PostForm("reason"), c.PostForm("description"), reportedUser, reportedEvent)
	c.Redirect(http.StatusFound, "/dashboard")
}
