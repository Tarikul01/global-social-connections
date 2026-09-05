package profile

import (
	"net/http"
	"strings"
	"time"

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

func profileOwnJSON(ctrl *Controller, c *gin.Context, p *Profile) gin.H {
	data := profileJSON(p)
	ctx := c.Request.Context()
	data["email"] = p.Email
	data["gender"] = p.Gender
	data["personal_quote"] = p.PersonalQuote
	data["pitch_url"] = p.PitchURL
	data["looking_for"] = p.LookingFor
	if data["looking_for"] == nil {
		data["looking_for"] = []string{}
	}
	data["profile_views"] = p.ProfileViews
	if p.ProfessionID != nil {
		data["profession_id"] = p.ProfessionID.String()
	} else {
		data["profession_id"] = ""
	}
	if p.ExperienceYears != nil {
		data["experience_years"] = *p.ExperienceYears
	}
	if p.Birthday != nil {
		data["birthday"] = p.Birthday.Format("2006-01-02")
		data["birthday_display"] = p.BirthdayDisplay
	} else {
		data["birthday"] = ""
		data["birthday_display"] = p.BirthdayDisplay
	}

	tags, _ := ctrl.svc.GetTags(ctx, p.UserID)
	tagNames := make([]string, 0, len(tags))
	for _, t := range tags {
		tagNames = append(tagNames, t.Name)
	}
	data["tags"] = tagNames

	skills, _ := ctrl.svc.GetSkills(ctx, p.UserID)
	skillNames := make([]string, 0, len(skills))
	for _, s := range skills {
		skillNames = append(skillNames, s.Name)
	}
	data["skills"] = skillNames

	links, _ := ctrl.svc.GetSocialLinks(ctx, p.UserID)
	linkMap := map[string]string{}
	for _, l := range links {
		linkMap[l.Platform] = l.URL
	}
	data["social_links"] = linkMap

	addr, _ := ctrl.svc.GetAddress(ctx, p.UserID)
	if addr.OK {
		data["city"] = addr.City
		data["region"] = addr.Region
		data["country"] = addr.Country
		data["lat"] = addr.Lat
		data["lng"] = addr.Lng
	}

	if ctrl.loc != nil {
		if lat, lng, live, ok := ctrl.loc.ActiveLiveOrPermanent(ctx, p.UserID); ok {
			// Prefer live for map pin indicator; keep address lat/lng as profile location above.
			data["live"] = live
			if live {
				data["live_lat"] = lat
				data["live_lng"] = lng
			} else if !addr.OK {
				data["lat"] = lat
				data["lng"] = lng
			}
		}
	}

	stats, _ := ctrl.svc.GetStats(ctx, p.UserID)
	data["followers"] = stats.Followers
	data["following"] = stats.Following
	data["views"] = stats.Views

	professions, _ := ctrl.svc.ListProfessions(ctx)
	profOut := make([]gin.H, 0, len(professions))
	for _, pr := range professions {
		profOut = append(profOut, gin.H{"id": pr.ID, "name": pr.Name, "slug": pr.Slug})
	}
	data["professions"] = profOut
	data["suggested_tags"] = suggestedTags
	data["suggested_skills"] = suggestedInterests
	data["suggested_looking_for"] = suggestedLookingFor
	data["social_platforms"] = socialPlatforms
	data["is_own"] = true
	return data
}

func profilePreviewJSON(p *Profile, followers, following int) gin.H {
	data := profileJSON(p)
	data["followers"] = followers
	data["following"] = following
	data["is_own"] = false
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
	c.JSON(http.StatusOK, gin.H{"success": true, "data": profileOwnJSON(ctrl, c, p)})
}

type updateProfileBody struct {
	FullName           string            `json:"full_name"`
	Bio                string            `json:"bio"`
	About              string            `json:"about"`
	Company            string            `json:"company"`
	JobTitle           string            `json:"job_title"`
	ProfessionID       string            `json:"profession_id"`
	ExperienceYears    *int              `json:"experience_years"`
	AvailabilityStatus string            `json:"availability"`
	LookingFor         []string          `json:"looking_for"`
	PitchURL           string            `json:"pitch_url"`
	PersonalQuote      string            `json:"personal_quote"`
	Gender             string            `json:"gender"`
	Birthday           string            `json:"birthday"`
	Tags               []string          `json:"tags"`
	Skills             []string          `json:"skills"`
	SocialLinks        map[string]string `json:"social_links"`
	Lat                *float64          `json:"lat"`
	Lng                *float64          `json:"lng"`
	City               string            `json:"city"`
	Region             string            `json:"region"`
	Country            string            `json:"country"`
}

// UpdateProfileJSON serves PUT /api/v1/me/profile
func (ctrl *Controller) UpdateProfileJSON(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "authentication required"})
		return
	}
	var body updateProfileBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid body"})
		return
	}
	fullName := strings.TrimSpace(body.FullName)
	if fullName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "full name is required"})
		return
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
		Company:            strings.TrimSpace(body.Company),
		JobTitle:           strings.TrimSpace(body.JobTitle),
		ExperienceYears:    body.ExperienceYears,
		ProfessionID:       profID,
		AvailabilityStatus: strings.TrimSpace(body.AvailabilityStatus),
		LookingFor:         normalizeLookingFor(body.LookingFor),
		PitchURL:           strings.TrimSpace(body.PitchURL),
		PersonalQuote:      strings.TrimSpace(body.PersonalQuote),
		Gender:             strings.TrimSpace(body.Gender),
	}
	if in.AvailabilityStatus == "" {
		in.AvailabilityStatus = "available"
	}
	switch in.AvailabilityStatus {
	case "available", "busy", "away", "offline", "open_to_work", "hiring":
	default:
		in.AvailabilityStatus = "available"
	}
	if bday := strings.TrimSpace(body.Birthday); bday != "" {
		if t, err := time.Parse("2006-01-02", bday); err == nil {
			in.Birthday = &t
		}
	}
	if err := ctrl.svc.Update(c.Request.Context(), u.ID, in); err != nil {
		ctrl.log.Error("profile update failed", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if body.Tags != nil {
		_ = ctrl.svc.ReplaceTags(c.Request.Context(), u.ID, body.Tags)
	}
	if body.Skills != nil {
		_ = ctrl.svc.ReplaceSkills(c.Request.Context(), u.ID, body.Skills)
	}
	if body.SocialLinks != nil {
		var links []SocialLink
		for _, platform := range socialPlatforms {
			url := strings.TrimSpace(body.SocialLinks[platform])
			if url != "" {
				links = append(links, SocialLink{Platform: platform, URL: url})
			}
		}
		_ = ctrl.svc.ReplaceSocialLinks(c.Request.Context(), u.ID, links)
	}
	if body.Lat != nil && body.Lng != nil && ctrl.loc != nil {
		_ = ctrl.loc.UpdateLocation(
			c.Request.Context(), u.ID, *body.Lat, *body.Lng,
			strings.TrimSpace(body.City), strings.TrimSpace(body.Region), strings.TrimSpace(body.Country),
		)
		if settings, err := ctrl.loc.GetSettings(c.Request.Context(), u.ID); err == nil {
			settings.DiscoveryEnabled = true
			settings.ShowOnMap = true
			if settings.SharingMode == "hidden" {
				settings.SharingMode = "discoverable"
			}
			_ = ctrl.loc.UpdateSettings(c.Request.Context(), u.ID, *settings)
		}
	}

	p, err := ctrl.svc.GetOwn(c.Request.Context(), u.ID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"updated": true}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": profileOwnJSON(ctrl, c, p)})
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
	c.JSON(http.StatusOK, gin.H{"success": true, "data": profileOwnJSON(ctrl, c, p)})
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

func normalizeLookingFor(raw []string) []string {
	if raw == nil {
		return nil
	}
	out := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, v := range raw {
		v = strings.TrimSpace(strings.ToLower(v))
		if v == "" || seen[v] || len(v) > 50 {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}