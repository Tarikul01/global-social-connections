package profile

import (
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/findneighbour/findneighbour/internal/auth"
	"github.com/findneighbour/findneighbour/internal/config"
	"github.com/findneighbour/findneighbour/internal/location"
	"github.com/findneighbour/findneighbour/internal/logger"
	"github.com/findneighbour/findneighbour/internal/storage"
	"github.com/findneighbour/findneighbour/internal/view"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var suggestedTags = []string{
	"Developer", "Investor", "Entrepreneur", "Designer", "Founder", "Student", "Freelancer",
}

var suggestedInterests = []string{
	"Nodejs", "Nestjs", "Reactjs", "Nextjs", "Go", "Python", "AWS", "Flutter", "TypeScript", "PostgreSQL",
}

var socialPlatforms = []string{
	"linkedin", "github", "twitter", "youtube", "instagram", "facebook", "website",
}

type Controller struct {
	svc     *Service
	storage *storage.Service
	loc     *location.Service
	authSvc *auth.Service
	cfg     *config.Config
	log     *logger.Logger
	tmpl    *template.Template
}

func NewController(svc *Service, storage *storage.Service, authSvc *auth.Service, cfg *config.Config, log *logger.Logger, tmpl *template.Template) *Controller {
	return &Controller{svc: svc, storage: storage, authSvc: authSvc, cfg: cfg, log: log, tmpl: tmpl}
}

func (ctrl *Controller) SetLocationService(loc *location.Service) {
	ctrl.loc = loc
}

func (ctrl *Controller) requireAuth(c *gin.Context) (*auth.User, bool) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return nil, false
	}
	return u, true
}

func (ctrl *Controller) profileViewData(c *gin.Context, p *Profile, isOwn bool) gin.H {
	ctx := c.Request.Context()
	tags, _ := ctrl.svc.GetTags(ctx, p.UserID)
	skills, _ := ctrl.svc.GetSkills(ctx, p.UserID)
	links, _ := ctrl.svc.GetSocialLinks(ctx, p.UserID)
	stats, _ := ctrl.svc.GetStats(ctx, p.UserID)
	friends, _ := ctrl.svc.ListFriends(ctx, p.UserID)
	addr, _ := ctrl.svc.GetAddress(ctx, p.UserID)

	role := p.ProfessionName
	if role == "" && len(tags) > 0 {
		role = tags[0].Name
	}
	quote := p.PersonalQuote
	if quote == "" {
		quote = "You are the architect of your own fate!"
	}
	initial := "?"
	if r := []rune(p.FullName); len(r) > 0 {
		initial = string(r[0])
	}

	data := gin.H{
		"Title":           p.FullName,
		"Profile":         p,
		"Tags":            tags,
		"Skills":          skills,
		"SocialLinks":     links,
		"Stats":           stats,
		"Friends":         friends,
		"Address":         addr,
		"IsOwn":           isOwn,
		"Role":            role,
		"MaskedEmail":     MaskEmail(p.Email),
		"Email":           p.Email,
		"Quote":           quote,
		"Initial":         initial,
		"Lat":             addr.Lat,
		"Lng":             addr.Lng,
		"City":            addr.City,
		"Region":          addr.Region,
		"SocialPlatforms": socialPlatforms,
	}
	return data
}

func (ctrl *Controller) Dashboard(c *gin.Context) {
	u, ok := ctrl.requireAuth(c)
	if !ok {
		return
	}
	p, _ := ctrl.svc.GetOwn(c.Request.Context(), u.ID)
	if p != nil && !p.OnboardingCompleted {
		c.Redirect(http.StatusFound, "/onboarding")
		return
	}
	view.Render(c, ctrl.tmpl, ctrl.cfg, "profile/dashboard.html", gin.H{"Title": "Dashboard", "Profile": p})
}

func (ctrl *Controller) ShowOwn(c *gin.Context) {
	u, ok := ctrl.requireAuth(c)
	if !ok {
		return
	}
	p, err := ctrl.svc.GetOwn(c.Request.Context(), u.ID)
	if err != nil {
		c.String(404, "Profile not found")
		return
	}
	if !p.OnboardingCompleted {
		c.Redirect(http.StatusFound, "/onboarding")
		return
	}
	data := ctrl.profileViewData(c, p, true)
	data["Title"] = "My Profile"
	view.Render(c, ctrl.tmpl, ctrl.cfg, "profile/show.html", data)
}

func (ctrl *Controller) ShowPublic(c *gin.Context) {
	username := c.Param("username")
	p, err := ctrl.svc.GetPublic(c.Request.Context(), username)
	if err != nil {
		c.String(404, "User not found")
		return
	}
	isOwn := false
	viewerID := uuid.Nil
	if u, ok := auth.CurrentUser(c); ok {
		viewerID = u.ID
		if u.ID == p.UserID {
			isOwn = true
		}
	}
	if !isOwn {
		_ = ctrl.svc.RecordView(c.Request.Context(), p.UserID, viewerID)
		p.ProfileViews++
	}
	view.Render(c, ctrl.tmpl, ctrl.cfg, "profile/show.html", ctrl.profileViewData(c, p, isOwn))
}

func (ctrl *Controller) EditForm(c *gin.Context) {
	u, ok := ctrl.requireAuth(c)
	if !ok {
		return
	}
	p, _ := ctrl.svc.GetOwn(c.Request.Context(), u.ID)
	professions, _ := ctrl.svc.ListProfessions(c.Request.Context())
	tags, _ := ctrl.svc.GetTags(c.Request.Context(), u.ID)
	skills, _ := ctrl.svc.GetSkills(c.Request.Context(), u.ID)
	links, _ := ctrl.svc.GetSocialLinks(c.Request.Context(), u.ID)
	tagSet := map[string]bool{}
	for _, t := range tags {
		tagSet[t.Name] = true
	}
	skillSet := map[string]bool{}
	suggestedSet := map[string]bool{}
	for _, name := range suggestedInterests {
		suggestedSet[name] = true
	}
	var customSkills []string
	for _, s := range skills {
		skillSet[s.Name] = true
		if !suggestedSet[s.Name] {
			customSkills = append(customSkills, s.Name)
		}
	}
	linkMap := map[string]string{}
	for _, l := range links {
		linkMap[l.Platform] = l.URL
	}
	addr, _ := ctrl.svc.GetAddress(c.Request.Context(), u.ID)
	lat, lng, city := 23.8103, 90.4125, ""
	if addr.OK {
		lat, lng, city = addr.Lat, addr.Lng, addr.City
	}
	view.Render(c, ctrl.tmpl, ctrl.cfg, "profile/edit.html", gin.H{
		"Title": "Edit Profile", "Profile": p, "Professions": professions,
		"SuggestedTags": suggestedTags, "TagSet": tagSet,
		"SuggestedInterests": suggestedInterests, "SkillSet": skillSet, "SkillNames": strings.Join(customSkills, ", "),
		"SocialPlatforms": socialPlatforms, "LinkMap": linkMap,
		"Lat": lat, "Lng": lng, "City": city, "Region": addr.Region, "Country": addr.Country,
	})
}

func (ctrl *Controller) Update(c *gin.Context) {
	u, ok := ctrl.requireAuth(c)
	if !ok {
		return
	}
	var profID *uuid.UUID
	if pid := c.PostForm("profession_id"); pid != "" {
		id, err := uuid.Parse(pid)
		if err == nil {
			profID = &id
		}
	}
	var exp *int
	if e := c.PostForm("experience_years"); e != "" {
		if v, err := strconv.Atoi(e); err == nil {
			exp = &v
		}
	}
	in := UpdateInput{
		FullName: c.PostForm("full_name"), Bio: c.PostForm("bio"), About: c.PostForm("about"),
		Company: c.PostForm("company"), JobTitle: c.PostForm("job_title"), ExperienceYears: exp,
		ProfessionID: profID, AvailabilityStatus: c.PostForm("availability_status"),
		PitchURL: strings.TrimSpace(c.PostForm("pitch_url")), PersonalQuote: strings.TrimSpace(c.PostForm("personal_quote")),
	}
	if in.AvailabilityStatus == "" {
		in.AvailabilityStatus = "available"
	}
	if err := ctrl.svc.Update(c.Request.Context(), u.ID, in); err != nil {
		view.Render(c, ctrl.tmpl, ctrl.cfg, "profile/edit.html", gin.H{"Title": "Edit Profile", "Error": err.Error(), "SuggestedTags": suggestedTags, "SuggestedInterests": suggestedInterests, "SocialPlatforms": socialPlatforms})
		return
	}
	_ = ctrl.svc.ReplaceTags(c.Request.Context(), u.ID, c.PostFormArray("tags"))

	interests := c.PostFormArray("interests")
	if custom := strings.TrimSpace(c.PostForm("interests_custom")); custom != "" {
		for _, part := range strings.Split(custom, ",") {
			interests = append(interests, strings.TrimSpace(part))
		}
	}
	_ = ctrl.svc.ReplaceSkills(c.Request.Context(), u.ID, interests)

	var links []SocialLink
	for _, platform := range socialPlatforms {
		url := strings.TrimSpace(c.PostForm("social_" + platform))
		if url != "" {
			links = append(links, SocialLink{Platform: platform, URL: url})
		}
	}
	_ = ctrl.svc.ReplaceSocialLinks(c.Request.Context(), u.ID, links)
	ctrl.savePermanentLocation(c, u.ID)
	c.Redirect(http.StatusFound, "/profile")
}

func (ctrl *Controller) savePermanentLocation(c *gin.Context, userID uuid.UUID) {
	if ctrl.loc == nil {
		return
	}
	lat, _ := strconv.ParseFloat(c.PostForm("lat"), 64)
	lng, _ := strconv.ParseFloat(c.PostForm("lng"), 64)
	if lat == 0 && lng == 0 {
		return
	}
	city := strings.TrimSpace(c.PostForm("city"))
	region := strings.TrimSpace(c.PostForm("region"))
	country := strings.TrimSpace(c.PostForm("country"))
	_ = ctrl.loc.UpdateLocation(c.Request.Context(), userID, lat, lng, city, region, country)
	settings, err := ctrl.loc.GetSettings(c.Request.Context(), userID)
	if err != nil {
		return
	}
	settings.DiscoveryEnabled = true
	settings.ShowOnMap = true
	if settings.SharingMode == "hidden" {
		settings.SharingMode = "discoverable"
	}
	_ = ctrl.loc.UpdateSettings(c.Request.Context(), userID, *settings)
}

func (ctrl *Controller) UploadPhoto(c *gin.Context) {
	u, ok := ctrl.requireAuth(c)
	if !ok {
		return
	}
	file, err := c.FormFile("photo")
	if err != nil {
		c.String(400, "No file uploaded")
		return
	}
	fileID, err := ctrl.storage.UploadProfilePhoto(c.Request.Context(), u.ID, file)
	if err != nil {
		c.String(400, err.Error())
		return
	}
	_ = ctrl.svc.SetProfilePhoto(c.Request.Context(), u.ID, fileID)
	c.Redirect(http.StatusFound, "/profile")
}

func (ctrl *Controller) OnboardingForm(c *gin.Context) {
	u, ok := ctrl.requireAuth(c)
	if !ok {
		return
	}
	p, _ := ctrl.svc.GetOwn(c.Request.Context(), u.ID)
	if p != nil && p.OnboardingCompleted {
		c.Redirect(http.StatusFound, "/")
		return
	}
	professions, _ := ctrl.svc.ListProfessions(c.Request.Context())
	view.Render(c, ctrl.tmpl, ctrl.cfg, "profile/onboarding.html", gin.H{
		"Title": "Complete your profile", "Profile": p, "Professions": professions,
		"SuggestedTags": suggestedTags, "Lat": 23.8103, "Lng": 90.4125,
	})
}

func (ctrl *Controller) CompleteOnboarding(c *gin.Context) {
	u, ok := ctrl.requireAuth(c)
	if !ok {
		return
	}
	var profID *uuid.UUID
	if pid := c.PostForm("profession_id"); pid != "" {
		id, _ := uuid.Parse(pid)
		profID = &id
	}
	lat, _ := strconv.ParseFloat(c.PostForm("lat"), 64)
	lng, _ := strconv.ParseFloat(c.PostForm("lng"), 64)
	if lat == 0 && lng == 0 {
		professions, _ := ctrl.svc.ListProfessions(c.Request.Context())
		view.Render(c, ctrl.tmpl, ctrl.cfg, "profile/onboarding.html", gin.H{
			"Title": "Complete your profile", "Error": "Please set your permanent address on the map.",
			"Professions": professions, "SuggestedTags": suggestedTags, "Lat": 23.8103, "Lng": 90.4125,
		})
		return
	}
	in := UpdateInput{
		FullName: c.PostForm("full_name"), Bio: c.PostForm("bio"), About: c.PostForm("about"),
		ProfessionID: profID, AvailabilityStatus: "available",
	}
	if in.FullName == "" && c.MustGet("user").(*auth.User).Username != "" {
		in.FullName = c.MustGet("user").(*auth.User).Username
	}
	_ = ctrl.svc.Update(c.Request.Context(), u.ID, in)
	_ = ctrl.svc.ReplaceTags(c.Request.Context(), u.ID, c.PostFormArray("tags"))
	ctrl.savePermanentLocation(c, u.ID)
	_ = ctrl.svc.CompleteOnboarding(c.Request.Context(), u.ID)
	c.Redirect(http.StatusFound, "/profile")
}
