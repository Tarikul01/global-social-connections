package controllers

import (
	"errors"
	"html/template"
	"net/http"
	"strconv"

	"github.com/findneighbour/findneighbour/internal/auth"
	"github.com/findneighbour/findneighbour/internal/config"
	"github.com/findneighbour/findneighbour/internal/discovery"
	"github.com/findneighbour/findneighbour/internal/logger"
	"github.com/findneighbour/findneighbour/internal/services"
	"github.com/findneighbour/findneighbour/internal/view"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UserController serves both Web MVC (HTML) and mobile JSON for users.
// Both paths call the same UserService / PostGIS queries.
type UserController struct {
	users *services.UserService
	cfg   *config.Config
	log   *logger.Logger
	tmpl  *template.Template
}

func NewUserController(users *services.UserService, cfg *config.Config, log *logger.Logger, tmpl *template.Template) *UserController {
	return &UserController{users: users, cfg: cfg, log: log, tmpl: tmpl}
}

// RenderNearbyWeb serves GET /explore — HTML page / HTMX fragment from shared Nearby().
func (ctrl *UserController) RenderNearbyWeb(c *gin.Context) {
	lat, _ := strconv.ParseFloat(c.DefaultQuery("lat", "23.8103"), 64)
	lng, _ := strconv.ParseFloat(c.DefaultQuery("lng", "90.4125"), 64)
	radius, _ := strconv.ParseFloat(c.DefaultQuery("radius_km", "5"), 64)

	viewerID := uuid.Nil
	if u, ok := auth.CurrentUser(c); ok {
		viewerID = u.ID
	}

	users, err := ctrl.users.Nearby(c.Request.Context(), viewerID, services.NearbyFilters{
		Lat: lat, Lng: lng, RadiusKm: radius,
		Profession: c.Query("profession"),
		Query:      c.Query("q"),
		Limit:      50,
	})
	if err != nil {
		ctrl.log.Error("nearby web query failed", "error", err)
		c.String(http.StatusInternalServerError, "Failed to load nearby people")
		return
	}
	total, online, _ := ctrl.users.CountDiscoverable(c.Request.Context())

	data := gin.H{
		"Title":        "Explore Nearby",
		"Users":        users,
		"TotalPeople":  total,
		"OnlinePeople": online,
		"Lat":          lat,
		"Lng":          lng,
		"RadiusKm":     radius,
	}

	// HTMX partial for list refresh; full page otherwise.
	if c.GetHeader("HX-Request") == "true" {
		view.Render(c, ctrl.tmpl, ctrl.cfg, "discovery/nearby_fragment.html", data)
		return
	}
	view.Render(c, ctrl.tmpl, ctrl.cfg, "discovery/index.html", data)
}

// GetNearbyJSON serves GET /api/v1/users/nearby — structured JSON for mobile clients.
func (ctrl *UserController) GetNearbyJSON(c *gin.Context) {
	lat, _ := strconv.ParseFloat(c.DefaultQuery("lat", "23.8103"), 64)
	lng, _ := strconv.ParseFloat(c.DefaultQuery("lng", "90.4125"), 64)
	radius, _ := strconv.ParseFloat(c.DefaultQuery("radius_km", "50"), 64)

	viewerID := uuid.Nil
	if u, ok := auth.CurrentUser(c); ok {
		viewerID = u.ID
	}

	onlineOnly := c.Query("status") == "online"
	offlineOnly := c.Query("status") == "offline"
	professions := c.QueryArray("profession")
	if len(professions) == 0 && c.Query("profession") != "" {
		professions = []string{c.Query("profession")}
	}

	users, err := ctrl.users.Nearby(c.Request.Context(), viewerID, services.NearbyFilters{
		Lat: lat, Lng: lng, RadiusKm: radius,
		Professions: professions,
		Query:       c.Query("q"),
		OnlineOnly:  onlineOnly,
		OfflineOnly: offlineOnly,
		Limit:       100,
	})
	if err != nil {
		ctrl.log.Error("nearby api query failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to load nearby users"})
		return
	}

	// Always include the logged-in user on the map (permanent address or live),
	// unless a status filter would exclude them.
	liveSharing := false
	if viewerID != uuid.Nil {
		if me, err := ctrl.users.MyMapMarker(c.Request.Context(), viewerID, lat, lng); err == nil && me != nil {
			liveSharing = me.Live
			filtered := users[:0]
			for _, u := range users {
				if u.ID != viewerID {
					filtered = append(filtered, u)
				}
			}
			users = filtered
			includeMe := true
			if offlineOnly && me.Online {
				includeMe = false
			}
			if onlineOnly && !me.Online {
				includeMe = false
			}
			if includeMe {
				users = append([]services.NearbyUser{*me}, users...)
			}
		}
	}

	total, online, _ := ctrl.users.CountDiscoverable(c.Request.Context())
	payload := gin.H{
		"total":  total,
		"online": online,
		"me":     viewerID != uuid.Nil,
		"live":   liveSharing,
	}
	if viewerID == uuid.Nil {
		// Guests receive avatar, username + designation (plus map coords / online for markers).
		payload["users"] = discovery.MapToGuestPublic(users)
	} else {
		payload["users"] = users
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    payload,
	})
}

// GetByIDJSON serves GET /api/v1/users/:id (login required for full profile).
func (ctrl *UserController) GetByIDJSON(c *gin.Context) {
	if _, ok := auth.CurrentUser(c); !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "login required"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid user id"})
		return
	}
	user, err := ctrl.users.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "user not found"})
			return
		}
		ctrl.log.Error("get user api failed", "error", err, "user_id", id)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to load user"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": user})
}

// ListProfessionsJSON serves GET /api/v1/professions — designations used on the map.
func (ctrl *UserController) ListProfessionsJSON(c *gin.Context) {
	list, err := ctrl.users.ListProfessions(c.Request.Context())
	if err != nil {
		ctrl.log.Error("list professions failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to load professions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}
