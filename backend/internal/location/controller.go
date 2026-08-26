package location

import (
	"html/template"
	"net/http"
	"strconv"

	"github.com/findneighbour/findneighbour/internal/auth"
	"github.com/findneighbour/findneighbour/internal/config"
	"github.com/findneighbour/findneighbour/internal/logger"
	"github.com/findneighbour/findneighbour/internal/view"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Controller struct {
	svc   *Service
	auth  *auth.Service
	cfg   *config.Config
	log   *logger.Logger
	tmpl  *template.Template
	bus   MapBroadcaster
}

// MapBroadcaster notifies live map clients that nearby data changed.
type MapBroadcaster interface {
	BroadcastMapChanged()
}

func NewController(svc *Service, auth *auth.Service, log *logger.Logger, tmpl *template.Template) *Controller {
	return &Controller{svc: svc, auth: auth, log: log, tmpl: tmpl}
}

func NewControllerWithConfig(svc *Service, auth *auth.Service, cfg *config.Config, log *logger.Logger, tmpl *template.Template) *Controller {
	return &Controller{svc: svc, auth: auth, cfg: cfg, log: log, tmpl: tmpl}
}

func (ctrl *Controller) SetBroadcaster(bus MapBroadcaster) {
	ctrl.bus = bus
}

func (ctrl *Controller) notifyMap() {
	if ctrl.bus != nil {
		ctrl.bus.BroadcastMapChanged()
	}
}

func (ctrl *Controller) SettingsForm(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	s, _ := ctrl.svc.GetSettings(c.Request.Context(), u.ID)
	view.Render(c, ctrl.tmpl, ctrl.cfg, "settings/location.html", gin.H{"Title": "Location Settings", "Settings": s})
}

func (ctrl *Controller) UpdateSettings(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	s := Settings{
		SharingMode: c.PostForm("sharing_mode"), DiscoveryEnabled: c.PostForm("discovery_enabled") == "on",
		LiveSharingEnabled: c.PostForm("live_sharing_enabled") == "on", LiveSharingScope: c.PostForm("live_sharing_scope"),
	}
	_ = ctrl.svc.UpdateSettings(c.Request.Context(), u.ID, s)
	c.Redirect(http.StatusFound, "/settings/location")
}

func (ctrl *Controller) UpdateLocation(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}
	lat, _ := strconv.ParseFloat(c.PostForm("lat"), 64)
	lng, _ := strconv.ParseFloat(c.PostForm("lng"), 64)
	if err := ctrl.svc.UpdateLocation(c.Request.Context(), u.ID, lat, lng, c.PostForm("city"), c.PostForm("region"), c.PostForm("country")); err != nil {
		c.JSON(500, gin.H{"error": "failed"})
		return
	}
	// Auto-enable discovery when user saves location
	settings, _ := ctrl.svc.GetSettings(c.Request.Context(), u.ID)
	settings.DiscoveryEnabled = true
	if settings.SharingMode == "hidden" {
		settings.SharingMode = "discoverable"
	}
	_ = ctrl.svc.UpdateSettings(c.Request.Context(), u.ID, *settings)
	c.JSON(200, gin.H{"ok": true})
}

func (ctrl *Controller) UpdateLiveLocation(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}
	lat, lng, eventRaw := parseLatLngEvent(c)
	var eventID *uuid.UUID
	if eventRaw != "" {
		id, err := uuid.Parse(eventRaw)
		if err == nil {
			eventID = &id
		}
	}
	if err := ctrl.svc.UpdateLiveLocation(c.Request.Context(), u.ID, eventID, lat, lng); err != nil {
		ctrl.log.Error("live location update failed", "error", err)
		c.JSON(500, gin.H{"error": "failed to update live location"})
		return
	}
	ctrl.notifyMap()
	c.JSON(200, gin.H{"ok": true, "live": true})
}

func (ctrl *Controller) StartLiveLocation(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}
	if err := ctrl.svc.EnableLiveSharing(c.Request.Context(), u.ID); err != nil {
		c.JSON(500, gin.H{"error": "failed to enable live location"})
		return
	}
	lat, lng, _ := parseLatLngEvent(c)
	if lat != 0 || lng != 0 {
		_ = ctrl.svc.UpdateLiveLocation(c.Request.Context(), u.ID, nil, lat, lng)
	}
	ctrl.notifyMap()
	c.JSON(200, gin.H{"ok": true, "live": true})
}

func parseLatLngEvent(c *gin.Context) (lat, lng float64, eventID string) {
	var body struct {
		Lat     float64 `json:"lat" form:"lat"`
		Lng     float64 `json:"lng" form:"lng"`
		EventID string  `json:"event_id" form:"event_id"`
	}
	_ = c.ShouldBind(&body)
	lat = body.Lat
	lng = body.Lng
	eventID = body.EventID
	if lat == 0 && lng == 0 {
		lat, _ = strconv.ParseFloat(c.PostForm("lat"), 64)
		lng, _ = strconv.ParseFloat(c.PostForm("lng"), 64)
	}
	if eventID == "" {
		eventID = c.PostForm("event_id")
	}
	return
}

func (ctrl *Controller) LiveStatus(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}
	c.JSON(200, gin.H{
		"ok":   true,
		"live": ctrl.svc.IsLiveSharing(c.Request.Context(), u.ID),
	})
}

func (ctrl *Controller) StopLiveLocation(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}
	var eventID *uuid.UUID
	if e := c.Query("event_id"); e != "" {
		id, _ := uuid.Parse(e)
		eventID = &id
	}
	_ = ctrl.svc.StopLiveLocation(c.Request.Context(), u.ID, eventID)
	ctrl.notifyMap()
	c.JSON(200, gin.H{"ok": true, "live": false})
}
