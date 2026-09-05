package controllers

import (
	"context"
	"errors"
	"html/template"
	"net/http"
	"strconv"

	"github.com/findneighbour/findneighbour/internal/auth"
	"github.com/findneighbour/findneighbour/internal/config"
	"github.com/findneighbour/findneighbour/internal/discovery"
	"github.com/findneighbour/findneighbour/internal/logger"
	"github.com/findneighbour/findneighbour/internal/realtime"
	"github.com/findneighbour/findneighbour/internal/services"
	"github.com/findneighbour/findneighbour/internal/view"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UserController serves both Web MVC (HTML) and mobile JSON for users.
// Both paths call the same UserService / PostGIS queries.
type UserController struct {
	users    *services.UserService
	cfg      *config.Config
	log      *logger.Logger
	tmpl     *template.Template
	presence realtime.Presence
}

func NewUserController(users *services.UserService, cfg *config.Config, log *logger.Logger, tmpl *template.Template) *UserController {
	return &UserController{users: users, cfg: cfg, log: log, tmpl: tmpl}
}

func (ctrl *UserController) SetPresence(p realtime.Presence) {
	ctrl.presence = p
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

	professions := c.QueryArray("professions")
	if len(professions) == 0 {
		professions = c.QueryArray("profession")
	}
	if len(professions) == 0 && c.Query("profession") != "" {
		professions = []string{c.Query("profession")}
	}

	payload, err := ctrl.NearbyPayload(c.Request.Context(), viewerID, NearbyQuery{
		Lat: lat, Lng: lng, RadiusKm: radius,
		Status:      c.Query("status"),
		Query:       c.Query("q"),
		Professions: professions,
	})
	if err != nil {
		ctrl.log.Error("nearby api query failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to load nearby users"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    payload,
	})
}

// NearbyQuery is the shared filter set for HTTP + WebSocket nearby snapshots.
type NearbyQuery struct {
	Lat         float64
	Lng         float64
	RadiusKm    float64
	Status      string
	Query       string
	Professions []string
}

// NearbyPayload builds the same payload used by REST and the realtime hub.
// Online = WebSocket heartbeat / last_seen (closes browser → offline).
func (ctrl *UserController) NearbyPayload(ctx context.Context, viewerID uuid.UUID, q NearbyQuery) (gin.H, error) {
	onlineOnly := q.Status == "online"
	offlineOnly := q.Status == "offline"
	if q.RadiusKm <= 0 {
		q.RadiusKm = 50
	}

	// Prefer SQL last_seen filters; presence overlay refines after fetch.
	sqlOnlineOnly := onlineOnly && ctrl.presence == nil
	sqlOfflineOnly := offlineOnly && ctrl.presence == nil

	users, err := ctrl.users.Nearby(ctx, viewerID, services.NearbyFilters{
		Lat: q.Lat, Lng: q.Lng, RadiusKm: q.RadiusKm,
		Professions: q.Professions,
		Query:       q.Query,
		OnlineOnly:  sqlOnlineOnly,
		OfflineOnly: sqlOfflineOnly,
		Limit:       100,
	})
	if err != nil {
		return nil, err
	}

	liveSharing := false
	if viewerID != uuid.Nil {
		if me, err := ctrl.users.MyMapMarker(ctx, viewerID, q.Lat, q.Lng); err == nil && me != nil {
			liveSharing = me.Live
			filtered := users[:0]
			for _, u := range users {
				if u.ID != viewerID {
					filtered = append(filtered, u)
				}
			}
			users = filtered
			users = append([]services.NearbyUser{*me}, users...)
		}
	}

	// Overlay DB heartbeat presence (source of truth for online/offline).
	if ctrl.presence != nil {
		out := users[:0]
		for _, u := range users {
			u.Online = ctrl.presence.IsOnline(u.ID)
			if onlineOnly && !u.Online {
				continue
			}
			if offlineOnly && u.Online {
				continue
			}
			out = append(out, u)
		}
		users = out
	}

	total, online, _ := ctrl.users.CountDiscoverable(ctx)

	payload := gin.H{
		"total":  total,
		"online": online,
		"me":     viewerID != uuid.Nil,
		"live":   liveSharing,
	}
	if viewerID == uuid.Nil {
		payload["users"] = discovery.MapToGuestPublic(users)
	} else {
		payload["users"] = users
	}
	return payload, nil
}

// NearbySnapshot implements realtime.NearbyProvider for the WebSocket hub.
func (ctrl *UserController) NearbySnapshot(viewerID uuid.UUID, sub realtime.SubscribeData) (any, error) {
	return ctrl.NearbyPayload(context.Background(), viewerID, NearbyQuery{
		Lat: sub.Lat, Lng: sub.Lng, RadiusKm: sub.RadiusKm,
		Status: sub.Status, Query: sub.Query, Professions: sub.Professions,
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

// StatsJSON serves GET /api/v1/stats — public discoverable counts for the map top bar.
func (ctrl *UserController) StatsJSON(c *gin.Context) {
	total, online, err := ctrl.users.CountDiscoverable(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to load stats"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"total":  total,
			"online": online,
		},
	})
}
