package event

import (
	"errors"
	"html/template"
	"net/http"
	"strconv"
	"strings"
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
	view.Render(c, ctrl.tmpl, ctrl.cfg, "events/new.html", gin.H{
		"Title":         "Create Event",
		"Lat":           23.8103,
		"Lng":           90.4125,
		"LocPickerMode": "event",
	})
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
	visibility := c.DefaultPostForm("visibility", "public")
	if visibility != "private" {
		visibility = "public"
	}
	created, err := ctrl.svc.Create(c.Request.Context(), Event{
		CreatorID: u.ID, Title: c.PostForm("title"), Description: c.PostForm("description"),
		Category: c.PostForm("category"), EventDate: date, StartTime: c.PostForm("start_time"),
		EndTime: c.PostForm("end_time"), LocationName: c.PostForm("location_name"),
		Latitude: lat, Longitude: lng, MaxParticipants: maxP,
		Visibility:          visibility,
		LiveLocationEnabled: c.PostForm("live_location_enabled") == "on",
	})
	if err != nil {
		view.Render(c, ctrl.tmpl, ctrl.cfg, "events/new.html", gin.H{
			"Title": "Create Event", "Error": err.Error(),
			"Lat": lat, "Lng": lng, "LocPickerMode": "event",
		})
		return
	}

	if invites := strings.TrimSpace(c.PostForm("invite_usernames")); invites != "" {
		for _, raw := range strings.Split(invites, ",") {
			q := strings.TrimSpace(raw)
			if q == "" {
				continue
			}
			invID, inviteeID, inviteeName, err := ctrl.svc.InviteByUsername(c.Request.Context(), created.ID, u.ID, q)
			if err != nil {
				continue
			}
			ctrl.notifyInvite(c, created, inviteeID, inviteeName, invID)
		}
	}

	c.Redirect(http.StatusFound, "/events/"+created.ID.String())
}

func (ctrl *Controller) Show(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.String(404, "Event not found")
		return
	}
	var viewerID uuid.UUID
	if u, ok := auth.CurrentUser(c); ok {
		viewerID = u.ID
	}
	invPage, _ := strconv.Atoi(c.DefaultQuery("inv_page", "1"))
	browsePage, _ := strconv.Atoi(c.DefaultQuery("browse_page", "1"))
	sc, err := ctrl.svc.ShowContext(c.Request.Context(), id, viewerID, invPage, browsePage)
	if err != nil {
		c.String(404, "Event not found")
		return
	}
	view.Render(c, ctrl.tmpl, ctrl.cfg, "events/show.html", gin.H{
		"Title":      sc.Event.Title,
		"Event":      sc.Event,
		"Ctx":        sc,
		"Message":    c.Query("msg"),
		"Error":      c.Query("error"),
		"InvPage":    invPage,
		"BrowsePage": browsePage,
	})
}

func (ctrl *Controller) SearchUsers(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}
	eventID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid event"})
		return
	}
	sc, err := ctrl.svc.ShowContext(c.Request.Context(), eventID, u.ID, 1, 1)
	if err != nil || !sc.IsHost {
		c.JSON(403, gin.H{"error": "forbidden"})
		return
	}
	users, err := ctrl.svc.SearchUsers(c.Request.Context(), eventID, u.ID, c.Query("q"))
	if err != nil {
		c.JSON(500, gin.H{"error": "search failed"})
		return
	}
	c.JSON(200, gin.H{"users": users})
}

func (ctrl *Controller) JoinRequest(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	eventID, _ := uuid.Parse(c.Param("id"))
	e, _ := ctrl.svc.Get(c.Request.Context(), eventID)
	if err := ctrl.svc.JoinRequest(c.Request.Context(), eventID, u.ID, c.PostForm("message")); err != nil {
		c.Redirect(http.StatusFound, "/events/"+eventID.String()+"?error="+err.Error())
		return
	}
	if ctrl.notif != nil && e != nil {
		_ = ctrl.notif.Create(c.Request.Context(), e.CreatorID, "event_join_request",
			"New join request for "+e.Title,
			u.Username+" wants to join your event",
			map[string]string{"event_id": eventID.String(), "user_id": u.ID.String()})
	}
	c.Redirect(http.StatusFound, "/events/"+eventID.String()+"?msg=Join+request+sent")
}

func (ctrl *Controller) AcceptJoin(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	eventID, _ := uuid.Parse(c.Param("id"))
	reqID, _ := uuid.Parse(c.Param("reqId"))
	e, _ := ctrl.svc.Get(c.Request.Context(), eventID)
	joinedID, err := ctrl.svc.AcceptJoin(c.Request.Context(), eventID, reqID, u.ID)
	if err != nil {
		c.Redirect(http.StatusFound, "/events/"+eventID.String()+"?error="+err.Error())
		return
	}
	if ctrl.notif != nil && joinedID != uuid.Nil && e != nil {
		_ = ctrl.notif.Create(c.Request.Context(), joinedID, "event_join_accepted",
			"You're in!",
			"Your request to join "+e.Title+" was accepted",
			map[string]string{"event_id": eventID.String()})
	}
	c.Redirect(http.StatusFound, "/events/"+eventID.String()+"?msg=Request+accepted")
}

func (ctrl *Controller) RejectJoin(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	eventID, _ := uuid.Parse(c.Param("id"))
	reqID, _ := uuid.Parse(c.Param("reqId"))
	if err := ctrl.svc.RejectJoin(c.Request.Context(), eventID, reqID, u.ID); err != nil {
		c.Redirect(http.StatusFound, "/events/"+eventID.String()+"?error="+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/events/"+eventID.String()+"?msg=Request+declined")
}

func (ctrl *Controller) Invite(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	eventID, _ := uuid.Parse(c.Param("id"))
	query := strings.TrimSpace(c.PostForm("username"))
	if query == "" {
		query = strings.TrimSpace(c.PostForm("query"))
	}
	e, _ := ctrl.svc.Get(c.Request.Context(), eventID)
	invID, inviteeID, inviteeName, err := ctrl.svc.InviteByUsername(c.Request.Context(), eventID, u.ID, query)
	if err != nil {
		c.Redirect(http.StatusFound, "/events/"+eventID.String()+"?error="+err.Error())
		return
	}
	ctrl.notifyInvite(c, e, inviteeID, inviteeName, invID)
	c.Redirect(http.StatusFound, "/events/"+eventID.String()+"?msg=Invitation+sent")
}

func (ctrl *Controller) InviteBatch(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	eventID, _ := uuid.Parse(c.Param("id"))
	e, _ := ctrl.svc.Get(c.Request.Context(), eventID)
	var ids []uuid.UUID
	for _, raw := range c.PostFormArray("user_ids") {
		if id, err := uuid.Parse(strings.TrimSpace(raw)); err == nil {
			ids = append(ids, id)
		}
	}
	var sent int
	var batchErrs []string
	seen := map[uuid.UUID]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		invID, name, err := ctrl.svc.InviteUser(c.Request.Context(), eventID, u.ID, id)
		if err != nil {
			batchErrs = append(batchErrs, err.Error())
			continue
		}
		ctrl.notifyInvite(c, e, id, name, invID)
		sent++
	}
	msg := "Sent " + strconv.Itoa(sent) + " invitation(s)"
	if len(batchErrs) > 0 {
		c.Redirect(http.StatusFound, "/events/"+eventID.String()+"?msg="+msg+"&error="+strings.Join(batchErrs, "; "))
		return
	}
	c.Redirect(http.StatusFound, "/events/"+eventID.String()+"?msg="+msg)
}

func (ctrl *Controller) PromoteHost(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	eventID, _ := uuid.Parse(c.Param("id"))
	targetID, _ := uuid.Parse(c.Param("userId"))
	if err := ctrl.svc.PromoteCoHost(c.Request.Context(), eventID, u.ID, targetID); err != nil {
		c.Redirect(http.StatusFound, "/events/"+eventID.String()+"?error="+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/events/"+eventID.String()+"?msg=Co-host+added")
}

func (ctrl *Controller) AcceptInvitation(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	eventID, _ := uuid.Parse(c.Param("id"))
	invID, _ := uuid.Parse(c.Param("invId"))
	e, _ := ctrl.svc.Get(c.Request.Context(), eventID)
	next := safeNext(c.PostForm("next"), "/events/"+eventID.String()+"?msg=You+joined+the+event")
	if err := ctrl.svc.AcceptInvitation(c.Request.Context(), eventID, invID, u.ID); err != nil {
		fail := safeNext(c.PostForm("next"), "/events/"+eventID.String())
		if strings.Contains(fail, "?") {
			c.Redirect(http.StatusFound, fail+"&error="+err.Error())
		} else {
			c.Redirect(http.StatusFound, fail+"?error="+err.Error())
		}
		return
	}
	if ctrl.notif != nil {
		_ = ctrl.notif.MarkReadByInvitation(c.Request.Context(), u.ID, invID)
		if e != nil {
			_ = ctrl.notif.Create(c.Request.Context(), e.CreatorID, "event_invite_accepted",
				"Invitation accepted",
				u.Username+" accepted your invite to "+e.Title,
				map[string]string{"event_id": eventID.String(), "user_id": u.ID.String()})
		}
	}
	if c.PostForm("next") == "/notifications" {
		c.Redirect(http.StatusFound, "/notifications?msg=You+joined+the+event")
		return
	}
	c.Redirect(http.StatusFound, next)
}

func (ctrl *Controller) DeclineInvitation(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	eventID, _ := uuid.Parse(c.Param("id"))
	invID, _ := uuid.Parse(c.Param("invId"))
	if err := ctrl.svc.DeclineInvitation(c.Request.Context(), invID, u.ID); err != nil {
		fail := safeNext(c.PostForm("next"), "/events/"+eventID.String())
		if strings.Contains(fail, "?") {
			c.Redirect(http.StatusFound, fail+"&error="+err.Error())
		} else {
			c.Redirect(http.StatusFound, fail+"?error="+err.Error())
		}
		return
	}
	if ctrl.notif != nil {
		_ = ctrl.notif.MarkReadByInvitation(c.Request.Context(), u.ID, invID)
	}
	if c.PostForm("next") == "/notifications" {
		c.Redirect(http.StatusFound, "/notifications?msg=Invitation+declined")
		return
	}
	c.Redirect(http.StatusFound, "/events/"+eventID.String()+"?msg=Invitation+declined")
}

func safeNext(next, fallback string) string {
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		return fallback
	}
	return next
}

func (ctrl *Controller) notifyInvite(c *gin.Context, e *Event, inviteeID uuid.UUID, inviteeName string, invID uuid.UUID) {
	if ctrl.notif == nil || inviteeID == uuid.Nil || invID == uuid.Nil {
		return
	}
	title := "Event invitation"
	body := "You were invited to an event"
	eventID := ""
	if e != nil {
		eventID = e.ID.String()
		if strings.TrimSpace(e.Title) != "" {
			body = "You were invited to " + e.Title
		}
	}
	data := map[string]string{
		"invitation_id": invID.String(),
		"invitee_name":  inviteeName,
	}
	if eventID != "" {
		data["event_id"] = eventID
	}
	if err := ctrl.notif.Create(c.Request.Context(), inviteeID, "event_invitation", title, body, data); err != nil {
		ctrl.log.Error("failed to create invite notification", "error", err, "invitee_id", inviteeID.String(), "invitation_id", invID.String())
	}
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

// ListHostedJSON returns published events where the current user is host/co-host.
func (ctrl *Controller) ListHostedJSON(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}
	list, err := ctrl.svc.ListHosted(c.Request.Context(), u.ID)
	if err != nil {
		ctrl.log.Error("list hosted events failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to load events"})
		return
	}
	if list == nil {
		list = []HostedEventSummary{}
	}
	createdCount, _ := ctrl.svc.CountByCreator(c.Request.Context(), u.ID)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    list,
		"meta": gin.H{
			"created_count": createdCount,
			"max_events":    MaxEventsPerCreator,
		},
	})
}

// EventMapJSON returns event pin + going members with locations (host or member).
func (ctrl *Controller) EventMapJSON(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}
	eventID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid event"})
		return
	}
	viewData, err := ctrl.svc.EventMapForHost(c.Request.Context(), eventID, u.ID)
	if err != nil {
		if errors.Is(err, ErrNotAuthorized) {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "forbidden"})
			return
		}
		ctrl.log.Error("event map members failed", "error", err, "event_id", eventID)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to load event map"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": viewData})
}

func (ctrl *Controller) SetLocationService(loc *location.Service) { ctrl.loc = loc }
