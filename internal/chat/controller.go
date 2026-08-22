package chat

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
	svc   *Service
	auth  *auth.Service
	notif *notification.Service
	cfg   *config.Config
	log   *logger.Logger
	tmpl  *template.Template
}

func NewController(svc *Service, auth *auth.Service, notif *notification.Service, cfg *config.Config, log *logger.Logger, tmpl *template.Template) *Controller {
	return &Controller{svc: svc, auth: auth, notif: notif, cfg: cfg, log: log, tmpl: tmpl}
}

func (ctrl *Controller) List(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	convs, _ := ctrl.svc.ListConversations(c.Request.Context(), u.ID)
	view.Render(c, ctrl.tmpl, ctrl.cfg, "chat/list.html", gin.H{"Title": "Messages", "Conversations": convs})
}

func (ctrl *Controller) Show(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	convID, _ := uuid.Parse(c.Param("id"))
	msgs, err := ctrl.svc.GetMessages(c.Request.Context(), convID, u.ID)
	if err != nil {
		c.String(403, "Not authorized")
		return
	}
	view.Render(c, ctrl.tmpl, ctrl.cfg, "chat/show.html", gin.H{
		"Title": "Chat", "Messages": msgs, "ConversationID": convID, "CurrentUserID": u.ID,
	})
}

func (ctrl *Controller) MessagesAPI(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}
	convID, _ := uuid.Parse(c.Param("id"))
	msgs, err := ctrl.svc.GetMessages(c.Request.Context(), convID, u.ID)
	if err != nil {
		c.JSON(403, gin.H{"error": "forbidden"})
		return
	}
	c.JSON(200, gin.H{"messages": msgs})
}

func (ctrl *Controller) SendMessage(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}
	convID, _ := uuid.Parse(c.Param("id"))
	msg, err := ctrl.svc.SendMessage(c.Request.Context(), convID, u.ID, c.PostForm("content"))
	if err != nil {
		c.JSON(403, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": msg})
}
