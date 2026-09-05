package profile

import (
	"net/http"
	"strings"

	"github.com/findneighbour/findneighbour/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func profileJSON(p *Profile) gin.H {
	if p == nil {
		return nil
	}
	return gin.H{
		"id":                   p.UserID,
		"username":             p.Username,
		"full_name":            p.FullName,
		"bio":                  p.Bio,
		"about":                p.About,
		"company":              p.Company,
		"job_title":            p.JobTitle,
		"profession":           p.ProfessionName,
		"photo_url":            p.ProfilePhotoURL,
		"cover_url":            p.CoverPhotoURL,
		"availability":         p.AvailabilityStatus,
		"onboarding_completed": p.OnboardingCompleted,
		"verified":             p.IsVerified,
	}
}

func profilePreviewJSON(p *Profile, followers, following int) gin.H {
	data := profileJSON(p)
	data["followers"] = followers
	data["following"] = following
	return data
}

// MeProfileJSON serves GET /api/v1/me/profile
func (ctrl *Controller) MeProfileJSON(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "authentication required"})
		return
	}
	p, err := ctrl.svc.GetOwn(c.Request.Context(), u.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "profile not found"})
		return
	}
	data := profileJSON(p)
	if ctrl.loc != nil {
		if lat, lng, live, ok := ctrl.loc.ActiveLiveOrPermanent(c.Request.Context(), u.ID); ok {
			data["lat"] = lat
			data["lng"] = lng
			data["live"] = live
		}
	}
	stats, _ := ctrl.svc.GetStats(c.Request.Context(), u.ID)
	data["followers"] = stats.Followers
	data["following"] = stats.Following
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// PublicProfileJSON serves GET /api/v1/profiles/:username (public preview + stats).
func (ctrl *Controller) PublicProfileJSON(c *gin.Context) {
	username := c.Param("username")
	p, err := ctrl.svc.GetPublic(c.Request.Context(), username)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "user not found"})
		return
	}
	stats, _ := ctrl.svc.GetStats(c.Request.Context(), p.UserID)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": profilePreviewJSON(p, stats.Followers, stats.Following)})
}

// UploadPhotoJSON serves POST /api/v1/me/photo (multipart field "photo").
func (ctrl *Controller) UploadPhotoJSON(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "authentication required"})
		return
	}
	file, err := c.FormFile("photo")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "photo file required"})
		return
	}
	fileID, err := ctrl.storage.UploadProfilePhoto(c.Request.Context(), u.ID, file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if err := ctrl.svc.SetProfilePhoto(c.Request.Context(), u.ID, fileID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to save photo"})
		return
	}
	p, err := ctrl.svc.GetOwn(c.Request.Context(), u.ID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"photo_url": ""}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": profileJSON(p)})
}

// OnboardingJSON serves GET /api/v1/me/onboarding — form options + current profile.
func (ctrl *Controller) OnboardingJSON(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "authentication required"})
		return
	}
	p, _ := ctrl.svc.GetOwn(c.Request.Context(), u.ID)
	if p != nil && p.OnboardingCompleted {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"completed": true,
				"profile":   profileJSON(p),
			},
		})
		return
	}
	professions, _ := ctrl.svc.ListProfessions(c.Request.Context())
	profOut := make([]gin.H, 0, len(professions))
	for _, pr := range professions {
		profOut = append(profOut, gin.H{"id": pr.ID, "name": pr.Name, "slug": pr.Slug})
	}
	data := gin.H{
		"completed":      false,
		"profile":        profileJSON(p),
		"professions":    profOut,
		"suggested_tags": suggestedTags,
		"lat":            23.8103,
		"lng":            90.4125,
	}
	if ctrl.loc != nil {
		if lat, lng, _, okLoc := ctrl.loc.ActiveLiveOrPermanent(c.Request.Context(), u.ID); okLoc {
			data["lat"] = lat
			data["lng"] = lng
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

type onboardingBody struct {
	FullName     string   `json:"full_name"`
	Bio          string   `json:"bio"`
	About        string   `json:"about"`
	ProfessionID string   `json:"profession_id"`
	Tags         []string `json:"tags"`
	Lat          float64  `json:"lat"`
	Lng          float64  `json:"lng"`
	City         string   `json:"city"`
	Region       string   `json:"region"`
	Country      string   `json:"country"`
}

// CompleteOnboardingJSON serves POST /api/v1/me/onboarding.
func (ctrl *Controller) CompleteOnboardingJSON(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "authentication required"})
		return
	}
	var body onboardingBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid body"})
		return
	}
	if body.Lat == 0 && body.Lng == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Please set your location on the map"})
		return
	}
	fullName := strings.TrimSpace(body.FullName)
	if fullName == "" {
		fullName = u.Username
	}
	var profID *uuid.UUID
	if pid := strings.TrimSpace(body.ProfessionID); pid != "" {
		id, err := uuid.Parse(pid)
		if err == nil {
			profID = &id
		}
	}
	in := UpdateInput{
		FullName:           fullName,
		Bio:                strings.TrimSpace(body.Bio),
		About:              strings.TrimSpace(body.About),
		ProfessionID:       profID,
		AvailabilityStatus: "available",
	}
	if err := ctrl.svc.Update(c.Request.Context(), u.ID, in); err != nil {
		ctrl.log.Error("onboarding profile update failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to save profile"})
		return
	}
	_ = ctrl.svc.ReplaceTags(c.Request.Context(), u.ID, body.Tags)

	if ctrl.loc != nil {
		_ = ctrl.loc.UpdateLocation(c.Request.Context(), u.ID, body.Lat, body.Lng, body.City, body.Region, body.Country)
		if settings, err := ctrl.loc.GetSettings(c.Request.Context(), u.ID); err == nil {
			settings.DiscoveryEnabled = true
			settings.ShowOnMap = true
			if settings.SharingMode == "hidden" {
				settings.SharingMode = "discoverable"
			}
			_ = ctrl.loc.UpdateSettings(c.Request.Context(), u.ID, *settings)
		}
	}

	if err := ctrl.svc.CompleteOnboarding(c.Request.Context(), u.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to complete onboarding"})
		return
	}
	p, _ := ctrl.svc.GetOwn(c.Request.Context(), u.ID)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": profileJSON(p)})
}
