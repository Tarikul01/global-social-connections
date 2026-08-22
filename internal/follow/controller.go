package follow

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

func (ctrl *Controller) Follow(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	targetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.String(400, "Invalid user")
		return
	}
	if err := ctrl.svc.Follow(c.Request.Context(), u.ID, targetID); err != nil {
		c.String(422, err.Error())
		return
	}
	c.Redirect(http.StatusFound, c.Request.Referer())
}

func (ctrl *Controller) Unfollow(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	targetID, _ := uuid.Parse(c.Param("id"))
	_ = ctrl.svc.Unfollow(c.Request.Context(), u.ID, targetID)
	c.Redirect(http.StatusFound, c.Request.Referer())
}

func (ctrl *Controller) Followers(c *gin.Context) {
	targetID, _ := uuid.Parse(c.Param("id"))
	list, _ := ctrl.svc.Followers(c.Request.Context(), targetID)
	c.JSON(200, list)
}

func (ctrl *Controller) Following(c *gin.Context) {
	targetID, _ := uuid.Parse(c.Param("id"))
	list, _ := ctrl.svc.Following(c.Request.Context(), targetID)
	c.JSON(200, list)
}
