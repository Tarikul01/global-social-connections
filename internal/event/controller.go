package event

import (
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/findneighbour/findneighbour/internal/auth"
	"github.com/findneighbour/findneighbour/internal/config"
	"github.com/findneighbour/findneighbour/internal/location"
	"github.com/findneighbour/findneighbour/internal/logger"
	"github.com/findneighbour/findneighbour/internal/notification"
	"github.com/findneighbour/findneighbour/internal/view"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Controller struct {
	svc   *Service
	loc   *location.Service
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
	events, _ := ctrl.svc.List(c.Request.Context())
	view.Render(c, ctrl.tmpl, ctrl.cfg, "events/list.html", gin.H{"Title": "Events", "Events": events})
}

func (ctrl *Controller) CreateForm(c *gin.Context) {
	view.Render(c, ctrl.tmpl, ctrl.cfg, "events/new.html", gin.H{"Title": "Create Event"})
}

func (ctrl *Controller) Create(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	maxP, _ := strconv.Atoi(c.DefaultPostForm("max_participants", "20"))
	lat, _ := strconv.ParseFloat(c.PostForm("lat"), 64)
	lng, _ := strconv.ParseFloat(c.PostForm("lng"), 64)
	date, _ := time.Parse("2006-01-02", c.PostForm("event_date"))
	created, err := ctrl.svc.Create(c.Request.Context(), Event{
		CreatorID: u.ID, Title: c.PostForm("title"), Description: c.PostForm("description"),
		Category: c.PostForm("category"), EventDate: date, StartTime: c.PostForm("start_time"),
		EndTime: c.PostForm("end_time"), LocationName: c.PostForm("location_name"),
		Latitude: lat, Longitude: lng, MaxParticipants: maxP,
		Visibility: c.DefaultPostForm("visibility", "public"),
		LiveLocationEnabled: c.PostForm("live_location_enabled") == "on",
	})
	if err != nil {
		view.Render(c, ctrl.tmpl, ctrl.cfg, "events/new.html", gin.H{"Title": "Create Event", "Error": err.Error()})
		return
	}
	c.Redirect(http.StatusFound, "/events/"+created.ID.String())
}

func (ctrl *Controller) Show(c *gin.Context) {
	id, _ := uuid.Parse(c.Param("id"))
	e, err := ctrl.svc.Get(c.Request.Context(), id)
	if err != nil {
		c.String(404, "Event not found")
		return
	}
	view.Render(c, ctrl.tmpl, ctrl.cfg, "events/show.html", gin.H{"Title": e.Title, "Event": e})
}

func (ctrl *Controller) JoinRequest(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	eventID, _ := uuid.Parse(c.Param("id"))
	_ = ctrl.svc.JoinRequest(c.Request.Context(), eventID, u.ID, c.PostForm("message"))
	c.Redirect(http.StatusFound, "/events/"+eventID.String())
}

func (ctrl *Controller) AcceptJoin(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	eventID, _ := uuid.Parse(c.Param("id"))
	reqID, _ := uuid.Parse(c.Param("reqId"))
	_ = ctrl.svc.AcceptJoin(c.Request.Context(), eventID, reqID, u.ID)
	c.Redirect(http.StatusFound, "/events/"+eventID.String())
}

func (ctrl *Controller) RejectJoin(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	eventID, _ := uuid.Parse(c.Param("id"))
	reqID, _ := uuid.Parse(c.Param("reqId"))
	_ = ctrl.svc.RejectJoin(c.Request.Context(), eventID, reqID, u.ID)
	c.Redirect(http.StatusFound, "/events/"+eventID.String())
}

func (ctrl *Controller) MapView(c *gin.Context) {
	id, _ := uuid.Parse(c.Param("id"))
	e, _ := ctrl.svc.Get(c.Request.Context(), id)
	view.Render(c, ctrl.tmpl, ctrl.cfg, "events/map.html", gin.H{"Title": "Event Map", "Event": e})
}

func (ctrl *Controller) ToggleLiveConsent(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}
	eventID, _ := uuid.Parse(c.Param("id"))
	consented := c.PostForm("consented") == "true"
	if err := ctrl.svc.ToggleLiveConsent(c.Request.Context(), eventID, u.ID, consented); err != nil {
		c.JSON(403, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func (ctrl *Controller) SetLocationService(loc *location.Service) { ctrl.loc = loc }
