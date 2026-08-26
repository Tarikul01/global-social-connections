package profile

import (
	"net/http"

	"github.com/findneighbour/findneighbour/internal/auth"
	"github.com/gin-gonic/gin"
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
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// PublicProfileJSON serves GET /api/v1/profiles/:username
func (ctrl *Controller) PublicProfileJSON(c *gin.Context) {
	username := c.Param("username")
	p, err := ctrl.svc.GetPublic(c.Request.Context(), username)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": profileJSON(p)})
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
