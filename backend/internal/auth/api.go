package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/findneighbour/findneighbour/internal/security"
	"github.com/findneighbour/findneighbour/internal/session"
	"github.com/gin-gonic/gin"
)

type loginBody struct {
	Email    string `json:"email" form:"email"`
	Password string `json:"password" form:"password"`
}

type registerBody struct {
	Email    string `json:"email" form:"email"`
	Username string `json:"username" form:"username"`
	Password string `json:"password" form:"password"`
	FullName string `json:"full_name" form:"full_name"`
}

func (ctrl *Controller) bindLogin(c *gin.Context) loginBody {
	var body loginBody
	_ = c.ShouldBind(&body)
	if body.Email == "" {
		body.Email = c.PostForm("email")
	}
	if body.Password == "" {
		body.Password = c.PostForm("password")
	}
	body.Email = strings.TrimSpace(body.Email)
	return body
}

func (ctrl *Controller) bindRegister(c *gin.Context) registerBody {
	var body registerBody
	_ = c.ShouldBind(&body)
	if body.Email == "" {
		body.Email = c.PostForm("email")
	}
	if body.Username == "" {
		body.Username = c.PostForm("username")
	}
	if body.Password == "" {
		body.Password = c.PostForm("password")
	}
	if body.FullName == "" {
		body.FullName = c.PostForm("full_name")
	}
	body.Email = strings.TrimSpace(body.Email)
	body.Username = strings.TrimSpace(body.Username)
	body.FullName = strings.TrimSpace(body.FullName)
	return body
}

func (ctrl *Controller) userPayload(u *User, csrf string) gin.H {
	return gin.H{
		"id":                   u.ID,
		"email":                u.Email,
		"username":             u.Username,
		"onboarding_completed": u.OnboardingCompleted,
		"csrf_token":           csrf,
	}
}

// LoginJSON handles POST /api/v1/auth/login
func (ctrl *Controller) LoginJSON(c *gin.Context) {
	body := ctrl.bindLogin(c)
	user, token, err := ctrl.svc.Login(c.Request.Context(), LoginInput{
		Email: body.Email, Password: body.Password,
	}, clientIP(c), c.Request.UserAgent())
	if err != nil {
		msg := "Invalid email or password"
		status := http.StatusUnauthorized
		if errors.Is(err, ErrAccountLocked) {
			msg = "Account temporarily locked. Try again later."
			status = http.StatusTooManyRequests
		} else if !errors.Is(err, ErrInvalidCredentials) {
			ctrl.log.Error("login failed", "error", err)
			msg = "Login failed. Please try again."
			status = http.StatusInternalServerError
		}
		c.JSON(status, gin.H{"success": false, "error": msg})
		return
	}
	ctrl.setSessionCookie(c, token)
	ctrl.log.Info("user logged in", "user_id", user.ID.String())
	csrf := ""
	if _, sess, err := ctrl.svc.ValidateSession(c.Request.Context(), token); err == nil && sess != nil {
		csrf = security.GenerateCSRFToken(ctrl.cfg.CSRFSecret, sess.ID.String())
	}
	ctrl.notifyMap()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    ctrl.userPayload(user, csrf),
	})
}

// RegisterJSON handles POST /api/v1/auth/register
func (ctrl *Controller) RegisterJSON(c *gin.Context) {
	body := ctrl.bindRegister(c)
	user, verifyToken, err := ctrl.svc.Register(c.Request.Context(), RegisterInput{
		Email: body.Email, Username: body.Username, Password: body.Password, FullName: body.FullName,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	ctrl.log.Info("user registered", "user_id", user.ID.String(), "verify_token", verifyToken)

	sessToken, err := ctrl.svc.IssueSession(c.Request.Context(), user.ID, clientIP(c), c.Request.UserAgent())
	if err != nil {
		c.JSON(http.StatusCreated, gin.H{
			"success": true,
			"data": gin.H{
				"id":                   user.ID,
				"email":                user.Email,
				"username":             user.Username,
				"onboarding_completed": false,
				"needs_login":          true,
			},
		})
		return
	}
	ctrl.setSessionCookie(c, sessToken)
	user.OnboardingCompleted = false
	ctrl.notifyMap()
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    ctrl.userPayload(user, ""),
	})
}

// LogoutJSON handles POST /api/v1/auth/logout
func (ctrl *Controller) LogoutJSON(c *gin.Context) {
	token, _ := c.Cookie(session.CookieName)
	if token != "" {
		_ = ctrl.svc.Logout(c.Request.Context(), token)
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(session.CookieName, "", -1, "/", "", ctrl.cfg.IsProduction, true)
	ctrl.notifyMap()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"logged_out": true}})
}

// MeJSON handles GET /api/v1/auth/me
func (ctrl *Controller) MeJSON(c *gin.Context) {
	u, ok := CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "authentication required"})
		return
	}
	csrf, _ := c.Get("csrf_token")
	csrfStr, _ := csrf.(string)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": ctrl.userPayload(u, csrfStr)})
}

// CSRFJSON handles GET /api/v1/auth/csrf
func (ctrl *Controller) CSRFJSON(c *gin.Context) {
	csrf, _ := c.Get("csrf_token")
	csrfStr, _ := csrf.(string)
	if csrfStr == "" {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"csrf_token": ""}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"csrf_token": csrfStr}})
}
