package profile

import (
	"html/template"
	"net/http"
	"strconv"

	"github.com/findneighbour/findneighbour/internal/auth"
	"github.com/findneighbour/findneighbour/internal/config"
	"github.com/findneighbour/findneighbour/internal/logger"
	"github.com/findneighbour/findneighbour/internal/storage"
	"github.com/findneighbour/findneighbour/internal/view"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Controller struct {
	svc     *Service
	storage *storage.Service
	authSvc *auth.Service
	cfg     *config.Config
	log     *logger.Logger
	tmpl    *template.Template
}

func NewController(svc *Service, storage *storage.Service, authSvc *auth.Service, cfg *config.Config, log *logger.Logger, tmpl *template.Template) *Controller {
	return &Controller{svc: svc, storage: storage, authSvc: authSvc, cfg: cfg, log: log, tmpl: tmpl}
}

func (ctrl *Controller) requireAuth(c *gin.Context) (*auth.User, bool) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/auth/login")
		return nil, false
	}
	return u, true
}

func (ctrl *Controller) Dashboard(c *gin.Context) {
	u, ok := ctrl.requireAuth(c)
	if !ok {
		return
	}
	p, _ := ctrl.svc.GetOwn(c.Request.Context(), u.ID)
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
	tags, _ := ctrl.svc.GetTags(c.Request.Context(), u.ID)
	skills, _ := ctrl.svc.GetSkills(c.Request.Context(), u.ID)
	view.Render(c, ctrl.tmpl, ctrl.cfg, "profile/show.html", gin.H{"Title": "My Profile", "Profile": p, "Tags": tags, "Skills": skills, "IsOwn": true})
}

func (ctrl *Controller) ShowPublic(c *gin.Context) {
	username := c.Param("username")
	p, err := ctrl.svc.GetPublic(c.Request.Context(), username)
	if err != nil {
		c.String(404, "User not found")
		return
	}
	tags, _ := ctrl.svc.GetTags(c.Request.Context(), p.UserID)
	skills, _ := ctrl.svc.GetSkills(c.Request.Context(), p.UserID)
	isOwn := false
	if u, ok := auth.CurrentUser(c); ok && u.ID == p.UserID {
		isOwn = true
	}
	view.Render(c, ctrl.tmpl, ctrl.cfg, "profile/show.html", gin.H{"Title": p.FullName, "Profile": p, "Tags": tags, "Skills": skills, "IsOwn": isOwn})
}

func (ctrl *Controller) EditForm(c *gin.Context) {
	u, ok := ctrl.requireAuth(c)
	if !ok {
		return
	}
	p, _ := ctrl.svc.GetOwn(c.Request.Context(), u.ID)
	professions, _ := ctrl.svc.ListProfessions(c.Request.Context())
	view.Render(c, ctrl.tmpl, ctrl.cfg, "profile/edit.html", gin.H{"Title": "Edit Profile", "Profile": p, "Professions": professions})
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
	}
	if err := ctrl.svc.Update(c.Request.Context(), u.ID, in); err != nil {
		view.Render(c, ctrl.tmpl, ctrl.cfg, "profile/edit.html", gin.H{"Title": "Edit Profile", "Error": err.Error()})
		return
	}
	for _, tag := range c.PostFormArray("tags") {
		_ = ctrl.svc.AddTag(c.Request.Context(), u.ID, tag)
	}
	for _, skill := range c.PostFormArray("skills") {
		_ = ctrl.svc.AddSkill(c.Request.Context(), u.ID, skill)
	}
	c.Redirect(http.StatusFound, "/profile")
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
	if _, ok := ctrl.requireAuth(c); !ok {
		return
	}
	professions, _ := ctrl.svc.ListProfessions(c.Request.Context())
	view.Render(c, ctrl.tmpl, ctrl.cfg, "profile/onboarding.html", gin.H{"Title": "Welcome", "Professions": professions})
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
	in := UpdateInput{FullName: c.PostForm("full_name"), Bio: c.PostForm("bio"), ProfessionID: profID}
	_ = ctrl.svc.Update(c.Request.Context(), u.ID, in)
	_ = ctrl.svc.CompleteOnboarding(c.Request.Context(), u.ID)
	c.Redirect(http.StatusFound, "/discover")
}
