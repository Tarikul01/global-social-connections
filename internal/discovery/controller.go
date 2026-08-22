package discovery

import (
	"html/template"
	"net/http"
	"strconv"

	"github.com/findneighbour/findneighbour/internal/auth"
	"github.com/findneighbour/findneighbour/internal/config"
	"github.com/findneighbour/findneighbour/internal/location"
	"github.com/findneighbour/findneighbour/internal/logger"
	"github.com/findneighbour/findneighbour/internal/view"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Controller struct {
	svc  *Service
	loc  *location.Service
	auth *auth.Service
	cfg  *config.Config
	log  *logger.Logger
	tmpl *template.Template
}

func NewController(svc *Service, loc *location.Service, auth *auth.Service, cfg *config.Config, log *logger.Logger, tmpl *template.Template) *Controller {
	return &Controller{svc: svc, loc: loc, auth: auth, cfg: cfg, log: log, tmpl: tmpl}
}

// LandingMap is the public homepage — OpenStreetMap with people. No login required.
func (ctrl *Controller) LandingMap(c *gin.Context) {
	if u, ok := auth.CurrentUser(c); ok && !u.OnboardingCompleted {
		c.Redirect(http.StatusFound, "/onboarding")
		return
	}
	total, online, _ := ctrl.svc.CountDiscoverable(c.Request.Context())
	loggedIn := false
	liveOn := false
	if u, ok := auth.CurrentUser(c); ok {
		loggedIn = true
		if ctrl.loc != nil {
			liveOn = ctrl.loc.IsLiveSharing(c.Request.Context(), u.ID)
		}
	}
	view.Render(c, ctrl.tmpl, ctrl.cfg, "landing.html", gin.H{
		"Title":        "FindNeighbour",
		"TotalPeople":  total,
		"OnlinePeople": online,
		"LoggedIn":     loggedIn,
		"LiveOn":       liveOn,
	})
}

func (ctrl *Controller) Index(c *gin.Context) {
	ctrl.LandingMap(c)
}

// PublicMapAPI returns privacy-safe markers for the public map (no login).
func (ctrl *Controller) PublicMapAPI(c *gin.Context) {
	lat, _ := strconv.ParseFloat(c.DefaultQuery("lat", "23.8103"), 64)
	lng, _ := strconv.ParseFloat(c.DefaultQuery("lng", "90.4125"), 64)
	radius, _ := strconv.ParseFloat(c.DefaultQuery("radius_km", "5000"), 64)

	viewerID := uuid.Nil
	if u, ok := auth.CurrentUser(c); ok {
		viewerID = u.ID
	}

	onlineOnly := c.Query("status") == "online"
	offlineOnly := c.Query("status") == "offline"

	users, err := ctrl.svc.Nearby(c.Request.Context(), viewerID, Filters{
		Lat: lat, Lng: lng, RadiusKm: radius,
		Profession:  c.Query("profession"),
		Query:       c.Query("q"),
		OnlineOnly:  onlineOnly,
		OfflineOnly: offlineOnly,
		Limit:       100,
	})
	if err != nil {
		ctrl.log.Error("map people query failed", "error", err)
		c.JSON(500, gin.H{"error": "failed to load people"})
		return
	}
	total, online, _ := ctrl.svc.CountDiscoverable(c.Request.Context())
	c.JSON(200, gin.H{
		"users":  users,
		"total":  total,
		"online": online,
	})
}

func (ctrl *Controller) NearbyAPI(c *gin.Context) {
	ctrl.PublicMapAPI(c)
}
