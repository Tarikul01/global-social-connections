package auth

import (
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/findneighbour/findneighbour/internal/config"
	"github.com/findneighbour/findneighbour/internal/logger"
	"github.com/findneighbour/findneighbour/internal/session"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Controller struct {
	svc  *Service
	cfg  *config.Config
	log  *logger.Logger
	tmpl *template.Template
	bus  MapBroadcaster
}

// MapBroadcaster pushes online/offline presence changes to map sockets.
type MapBroadcaster interface {
	BroadcastMapChanged()
}

func NewController(svc *Service, cfg *config.Config, log *logger.Logger, tmpl *template.Template) *Controller {
	return &Controller{svc: svc, cfg: cfg, log: log, tmpl: tmpl}
}

func (ctrl *Controller) SetBroadcaster(bus MapBroadcaster) {
	ctrl.bus = bus
}

func (ctrl *Controller) notifyMap() {
	if ctrl.bus != nil {
		ctrl.bus.BroadcastMapChanged()
	}
}

func (ctrl *Controller) render(c *gin.Context, name string, data gin.H) {
	if data == nil {
		data = gin.H{}
	}
	data["AppURL"] = ctrl.cfg.AppURL
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := ctrl.tmpl.ExecuteTemplate(c.Writer, name, data); err != nil {
		ctrl.log.Error("template render failed", "template", name, "error", err)
		c.String(http.StatusInternalServerError, "Page failed to render")
	}
}

func (ctrl *Controller) LoginForm(c *gin.Context) {
	ctrl.render(c, "auth/login.html", gin.H{
		"Title": "Login",
		"Next":  c.Query("next"),
	})
}

func (ctrl *Controller) RegisterForm(c *gin.Context) {
	ctrl.render(c, "auth/register.html", gin.H{
		"Title": "Register",
		"Next":  c.Query("next"),
	})
}

func (ctrl *Controller) Login(c *gin.Context) {
	in := LoginInput{
		Email:    c.PostForm("email"),
		Password: c.PostForm("password"),
	}
	user, token, err := ctrl.svc.Login(c.Request.Context(), in, clientIP(c), c.Request.UserAgent())
	if err != nil {
		msg := "Invalid email or password"
		if errors.Is(err, ErrAccountLocked) {
			msg = "Account temporarily locked. Try again later."
		} else if !errors.Is(err, ErrInvalidCredentials) {
			ctrl.log.Error("login failed", "error", err)
			msg = "Login failed. Please try again."
		}
		ctrl.render(c, "auth/login.html", gin.H{
			"Title": "Login",
			"Error": msg,
			"Next":  c.PostForm("next"),
		})
		return
	}
	ctrl.setSessionCookie(c, token)
	ctrl.log.Info("user logged in", "user_id", user.ID.String())
	if !user.OnboardingCompleted {
		c.Redirect(http.StatusFound, "/onboarding")
		return
	}
	next := c.PostForm("next")
	if next == "" {
		next = c.Query("next")
	}
	if u, err := url.QueryUnescape(next); err == nil {
		next = u
	}
	if next == "" || next[0] != '/' {
		next = "/"
	}
	c.Redirect(http.StatusFound, next)
}

func (ctrl *Controller) Register(c *gin.Context) {
	in := RegisterInput{
		Email:    c.PostForm("email"),
		Username: c.PostForm("username"),
		Password: c.PostForm("password"),
		FullName: c.PostForm("full_name"),
	}
	user, verifyToken, err := ctrl.svc.Register(c.Request.Context(), in)
	if err != nil {
		ctrl.render(c, "auth/register.html", gin.H{
			"Title": "Register",
			"Error": err.Error(),
			"Next":  c.PostForm("next"),
		})
		return
	}
	ctrl.log.Info("user registered", "user_id", user.ID.String(), "verify_token", verifyToken)

	// Auto-login and send to profile setup (permanent address, tags, about).
	sessToken, err := ctrl.svc.IssueSession(c.Request.Context(), user.ID, clientIP(c), c.Request.UserAgent())
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login?registered=1")
		return
	}
	ctrl.setSessionCookie(c, sessToken)
	c.Redirect(http.StatusFound, "/onboarding")
}

func (ctrl *Controller) Logout(c *gin.Context) {
	token, _ := c.Cookie(session.CookieName)
	if token != "" {
		_ = ctrl.svc.Logout(c.Request.Context(), token)
	}
	c.SetCookie(session.CookieName, "", -1, "/", "", ctrl.cfg.IsProduction, true)
	c.Redirect(http.StatusFound, "/")
}

func (ctrl *Controller) VerifyEmail(c *gin.Context) {
	token := c.Query("token")
	if err := ctrl.svc.VerifyEmail(c.Request.Context(), token); err != nil {
		c.String(http.StatusBadRequest, "Invalid or expired verification link")
		return
	}
	c.Redirect(http.StatusFound, "/auth/login?verified=1")
}

func (ctrl *Controller) setSessionCookie(c *gin.Context, token string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(session.CookieName, token, int(ctrl.cfg.SessionTTL.Seconds()), "/", "", ctrl.cfg.IsProduction, true)
}

func CurrentUser(c *gin.Context) (*User, bool) {
	v, ok := c.Get("user")
	if !ok {
		return nil, false
	}
	u, ok := v.(*User)
	return u, ok
}

func CurrentUserID(c *gin.Context) (uuid.UUID, bool) {
	u, ok := CurrentUser(c)
	if !ok {
		return uuid.Nil, false
	}
	return u.ID, true
}

// clientIP returns a value safe for Postgres INET (empty → omit as NULL).
func clientIP(c *gin.Context) string {
	candidates := []string{
		c.ClientIP(),
		c.GetHeader("X-Vercel-Forwarded-For"),
		c.GetHeader("X-Real-IP"),
		c.GetHeader("X-Forwarded-For"),
	}
	for _, raw := range candidates {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if i := strings.IndexByte(raw, ','); i >= 0 {
			raw = strings.TrimSpace(raw[:i])
		}
		if raw != "" && raw != "::" && raw != "0.0.0.0" {
			return raw
		}
	}
	return ""
}
