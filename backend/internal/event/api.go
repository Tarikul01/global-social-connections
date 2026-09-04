package event

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/findneighbour/findneighbour/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func eventJSON(e Event) gin.H {
	return gin.H{
		"id":                    e.ID,
		"creator_id":            e.CreatorID,
		"title":                 e.Title,
		"description":           e.Description,
		"category":              e.Category,
		"event_date":            e.EventDate.Format("2006-01-02"),
		"event_date_label":      e.EventDate.Format("Jan 2, 2006"),
		"start_time":            trimTime(e.StartTime),
		"end_time":              trimTime(e.EndTime),
		"location_name":         e.LocationName,
		"lat":                   e.Latitude,
		"lng":                   e.Longitude,
		"max_participants":      e.MaxParticipants,
		"visibility":            e.Visibility,
		"status":                e.Status,
		"live_location_enabled": e.LiveLocationEnabled,
		"member_count":          e.MemberCount,
	}
}

func trimTime(t string) string {
	t = strings.TrimSpace(t)
	if len(t) >= 5 {
		return t[:5]
	}
	return t
}

func showContextJSON(sc *ShowContext) gin.H {
	invID := ""
	if sc.MyInvitationID != nil {
		invID = sc.MyInvitationID.String()
	}
	requests := make([]gin.H, 0, len(sc.PendingJoinRequests))
	for _, r := range sc.PendingJoinRequests {
		requests = append(requests, gin.H{
			"id":        r.ID,
			"user_id":   r.UserID,
			"username":  r.Username,
			"full_name": r.FullName,
			"message":   r.Message,
		})
	}
	hosts := make([]gin.H, 0, len(sc.Hosts))
	for _, h := range sc.Hosts {
		hosts = append(hosts, gin.H{
			"user_id":    h.UserID,
			"username":   h.Username,
			"full_name":  h.FullName,
			"role":       h.Role,
			"is_creator": h.IsCreator,
		})
	}
	return gin.H{
		"event":                eventJSON(sc.Event),
		"is_creator":           sc.IsCreator,
		"is_host":              sc.IsHost,
		"is_member":            sc.IsMember,
		"has_pending_join":     sc.HasPendingJoin,
		"my_invitation_id":     invID,
		"pending_join_requests": requests,
		"hosts":                hosts,
		"accepted_invites":     sc.AcceptedInvites,
		"pending_invites":      sc.PendingInvites,
		"total_invites":        sc.TotalInvites,
	}
}

// ListJSON returns published events for the discover list.
func (ctrl *Controller) ListJSON(c *gin.Context) {
	list, err := ctrl.svc.List(c.Request.Context())
	if err != nil {
		ctrl.log.Error("list events failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to load events"})
		return
	}
	out := make([]gin.H, 0, len(list))
	for _, e := range list {
		out = append(out, eventJSON(e))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": out})
}

type createEventBody struct {
	Title               string  `json:"title"`
	Description         string  `json:"description"`
	Category            string  `json:"category"`
	EventDate           string  `json:"event_date"`
	StartTime           string  `json:"start_time"`
	EndTime             string  `json:"end_time"`
	LocationName        string  `json:"location_name"`
	Lat                 float64 `json:"lat"`
	Lng                 float64 `json:"lng"`
	MaxParticipants     int     `json:"max_participants"`
	Visibility          string  `json:"visibility"`
	LiveLocationEnabled bool    `json:"live_location_enabled"`
	InviteUsernames     string  `json:"invite_usernames"`
}

// CreateJSON creates an event from JSON body.
func (ctrl *Controller) CreateJSON(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}
	var body createEventBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid body"})
		return
	}
	date, err := time.Parse("2006-01-02", strings.TrimSpace(body.EventDate))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "event_date must be YYYY-MM-DD"})
		return
	}
	visibility := body.Visibility
	if visibility != "private" {
		visibility = "public"
	}
	maxP := body.MaxParticipants
	if maxP <= 0 {
		maxP = 20
	}
	created, err := ctrl.svc.Create(c.Request.Context(), Event{
		CreatorID:           u.ID,
		Title:               strings.TrimSpace(body.Title),
		Description:         strings.TrimSpace(body.Description),
		Category:            strings.TrimSpace(body.Category),
		EventDate:           date,
		StartTime:           strings.TrimSpace(body.StartTime),
		EndTime:             strings.TrimSpace(body.EndTime),
		LocationName:        strings.TrimSpace(body.LocationName),
		Latitude:            body.Lat,
		Longitude:           body.Lng,
		MaxParticipants:     maxP,
		Visibility:          visibility,
		LiveLocationEnabled: body.LiveLocationEnabled,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	if invites := strings.TrimSpace(body.InviteUsernames); invites != "" {
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

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": eventJSON(*created)})
}

// ShowJSON returns event detail + viewer membership context.
func (ctrl *Controller) ShowJSON(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid event"})
		return
	}
	var viewerID uuid.UUID
	if u, ok := auth.CurrentUser(c); ok {
		viewerID = u.ID
	}
	sc, err := ctrl.svc.ShowContext(c.Request.Context(), id, viewerID, 1, 1)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "event not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": showContextJSON(sc)})
}

// JoinJSON requests to join a public event.
func (ctrl *Controller) JoinJSON(c *gin.Context) {
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
	var body struct {
		Message string `json:"message"`
	}
	_ = c.ShouldBindJSON(&body)
	e, _ := ctrl.svc.Get(c.Request.Context(), eventID)
	if err := ctrl.svc.JoinRequest(c.Request.Context(), eventID, u.ID, body.Message); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, ErrPrivateEvent) || errors.Is(err, ErrNotAuthorized) {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"success": false, "error": err.Error()})
		return
	}
	if ctrl.notif != nil && e != nil {
		_ = ctrl.notif.Create(c.Request.Context(), e.CreatorID, "event_join_request",
			"New join request for "+e.Title,
			u.Username+" wants to join your event",
			map[string]string{"event_id": eventID.String(), "user_id": u.ID.String()})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "requested"}})
}

// AcceptInvitationJSON accepts an invitation.
func (ctrl *Controller) AcceptInvitationJSON(c *gin.Context) {
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
	invID, err := uuid.Parse(c.Param("invId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid invitation"})
		return
	}
	e, _ := ctrl.svc.Get(c.Request.Context(), eventID)
	if err := ctrl.svc.AcceptInvitation(c.Request.Context(), eventID, invID, u.ID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
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
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "joined"}})
}

// DeclineInvitationJSON declines an invitation.
func (ctrl *Controller) DeclineInvitationJSON(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}
	invID, err := uuid.Parse(c.Param("invId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid invitation"})
		return
	}
	if err := ctrl.svc.DeclineInvitation(c.Request.Context(), invID, u.ID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "declined"}})
}

// AcceptJoinJSON accepts a pending join request (host).
func (ctrl *Controller) AcceptJoinJSON(c *gin.Context) {
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
	reqID, err := uuid.Parse(c.Param("reqId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request"})
		return
	}
	e, _ := ctrl.svc.Get(c.Request.Context(), eventID)
	joinedID, err := ctrl.svc.AcceptJoin(c.Request.Context(), eventID, reqID, u.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if ctrl.notif != nil && joinedID != uuid.Nil && e != nil {
		_ = ctrl.notif.Create(c.Request.Context(), joinedID, "event_join_accepted",
			"You're in!",
			"Your request to join "+e.Title+" was accepted",
			map[string]string{"event_id": eventID.String()})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "accepted"}})
}

// RejectJoinJSON rejects a pending join request (host).
func (ctrl *Controller) RejectJoinJSON(c *gin.Context) {
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
	reqID, err := uuid.Parse(c.Param("reqId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request"})
		return
	}
	if err := ctrl.svc.RejectJoin(c.Request.Context(), eventID, reqID, u.ID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "rejected"}})
}

// InviteJSON invites a user by username (host).
func (ctrl *Controller) InviteJSON(c *gin.Context) {
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
	var body struct {
		Username string `json:"username"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Username) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "username required"})
		return
	}
	e, _ := ctrl.svc.Get(c.Request.Context(), eventID)
	invID, inviteeID, inviteeName, err := ctrl.svc.InviteByUsername(c.Request.Context(), eventID, u.ID, body.Username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	ctrl.notifyInvite(c, e, inviteeID, inviteeName, invID)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"invitation_id": invID, "invitee": inviteeName}})
}
