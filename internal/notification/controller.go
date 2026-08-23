package notification

import (
	"context"
	"html/template"
	"net/http"

	"github.com/findneighbour/findneighbour/internal/auth"
	"github.com/findneighbour/findneighbour/internal/config"
	"github.com/findneighbour/findneighbour/internal/logger"
	"github.com/findneighbour/findneighbour/internal/view"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type inviteStatusFn func(ctx context.Context, invID, inviteeID uuid.UUID) (string, error)

type Controller struct {
	svc          *Service
	auth         *auth.Service
	cfg          *config.Config
	log          *logger.Logger
	tmpl         *template.Template
	inviteStatus inviteStatusFn
}

// Item is a notification enriched for the list template.
type Item struct {
	Notification
	Unread        bool
	InvitePending bool
	EventID       string
	InvitationID  string
}

func NewController(svc *Service, auth *auth.Service, log *logger.Logger, tmpl *template.Template) *Controller {
	return &Controller{svc: svc, auth: auth, log: log, tmpl: tmpl}
}

func NewControllerWithConfig(svc *Service, auth *auth.Service, cfg *config.Config, log *logger.Logger, tmpl *template.Template) *Controller {
	return &Controller{svc: svc, auth: auth, cfg: cfg, log: log, tmpl: tmpl}
}

func (ctrl *Controller) SetInviteStatusChecker(fn inviteStatusFn) {
	ctrl.inviteStatus = fn
}

func (ctrl *Controller) List(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	list, _ := ctrl.svc.List(c.Request.Context(), u.ID)
	items := make([]Item, 0, len(list))
	for _, n := range list {
		item := Item{
			Notification: n,
			Unread:       n.ReadAt == nil,
		}
		if n.Data != nil {
			item.EventID = n.Data["event_id"]
			item.InvitationID = n.Data["invitation_id"]
		}
		if n.Type == "event_invitation" && item.InvitationID != "" && ctrl.inviteStatus != nil {
			invID, err := uuid.Parse(item.InvitationID)
			if err == nil {
				status, err := ctrl.inviteStatus(c.Request.Context(), invID, u.ID)
				if err == nil && status == "pending" {
					item.InvitePending = true
				}
			}
		}
		items = append(items, item)
	}
	view.Render(c, ctrl.tmpl, ctrl.cfg, "notifications/list.html", gin.H{
		"Title":         "Notifications",
		"Notifications": items,
		"Msg":           c.Query("msg"),
		"Error":         c.Query("error"),
	})
}

func (ctrl *Controller) MarkRead(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}
	id, _ := uuid.Parse(c.Param("id"))
	_ = ctrl.svc.MarkRead(c.Request.Context(), id, u.ID)
	next := c.PostForm("next")
	if next == "" {
		next = "/notifications"
	}
	c.Redirect(http.StatusFound, next)
}
