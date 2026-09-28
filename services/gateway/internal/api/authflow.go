package api

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"astralis/gateway/internal/auth"
	"astralis/gateway/internal/mail"

	"github.com/gin-gonic/gin"
)

// ------------------------------------------------------------------ start

type authStartReq struct {
	Email string `json:"email" binding:"required,max=254"`
}

// handleAuthStart sends a sign-in code.
//
// Sign-up and sign-in are one flow, deliberately: there is no "that email is
// not registered" response, so the form cannot be used to discover who has an
// account here. For a mental-health tool, whether a given address belongs to
// a user is itself sensitive.
func (s *Server) handleAuthStart(c *gin.Context) {
	var req authStartReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_email", "message": auth.ErrInvalidEmail.Error()})
		return
	}
	email, err := auth.NormalizeEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_email", "message": err.Error()})
		return
	}

	ctx := c.Request.Context()
	n, err := s.store.CodesSince(ctx, email, time.Now().Add(-time.Hour))
	if err != nil {
		fail(c, err, "could not send a code")
		return
	}
	if n >= auth.MaxCodesPerHour {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":   "too_many_codes",
			"message": "Several codes have gone to that address in the last hour. Use the most recent one, or try again later.",
		})
		return
	}

	code, err := auth.NewCode()
	if err != nil {
		fail(c, err, "could not send a code")
		return
	}
	if err := s.store.IssueLoginCode(ctx, email, auth.HashCode(s.cfg.AuthSecret, email, code), auth.CodeTTL); err != nil {
		fail(c, err, "could not send a code")
		return
	}
	if err := s.mailer.Send(ctx, mail.SignInCode(email, code)); err != nil {
		// Masked address only. A raw email in a log line is exactly the leak
		// this whole design is trying to prevent.
		slog.Error("send code", "to", auth.MaskEmail(email), "err", err)
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "mail_failed",
			"message": "The code could not be sent just now. Try again in a minute.",
		})
		return
	}

	resp := gin.H{"sent": true}
	if s.cfg.DevEchoCode && !s.cfg.Production() {
		resp["devCode"] = code // config refuses to boot with this in production
	}
	c.JSON(http.StatusOK, resp)
}

// ------------------------------------------------------------------ verify

type authVerifyReq struct {
	Email          string `json:"email" binding:"required,max=254"`
	Code           string `json:"code" binding:"required"`
	NotifyReleases bool   `json:"notifyReleases"`
}

func (s *Server) handleAuthVerify(c *gin.Context) {
	var req authVerifyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_code", "message": "Enter the 6-digit code from the email."})
		return
	}
	email, err := auth.NormalizeEmail(req.Email)
	code := strings.TrimSpace(req.Code)
	if err != nil || len(code) != 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_code", "message": "Enter the 6-digit code from the email."})
		return
	}

	ctx := c.Request.Context()
	ok, err := s.store.ConsumeLoginCode(ctx, email, auth.HashCode(s.cfg.AuthSecret, email, code), auth.MaxCodeGuesses)
	if err != nil {
		fail(c, err, "could not check that code")
		return
	}
	if !ok {
		// One message for wrong, expired, used and burned. Distinguishing them
		// helps an attacker far more than it helps a person.
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_code",
			"message": "That code is wrong or has expired. Check the latest email, or request a new code.",
		})
		return
	}

	u, err := s.store.UserForVerifiedEmail(ctx, email, req.NotifyReleases)
	if err != nil {
		fail(c, err, "could not sign you in")
		return
	}
	token, err := auth.NewToken()
	if err != nil {
		fail(c, err, "could not sign you in")
		return
	}
	if err := s.store.CreateAuthSession(ctx, u.ID, auth.HashToken(token), auth.SessionTTL); err != nil {
		fail(c, err, "could not sign you in")
		return
	}
	s.setSessionCookie(c, token)
	c.JSON(http.StatusOK, gin.H{"signedIn": true})
}

// ------------------------------------------------------------------ sign out

// handleSignOut deletes the session row, which ends it everywhere that token
// was ever used — then clears the cookie. Works even with a dead cookie, so
// "sign out" always results in being signed out.
func (s *Server) handleSignOut(c *gin.Context) {
	if token, err := c.Cookie(auth.CookieName); err == nil && token != "" {
		if err := s.store.DeleteAuthSession(c.Request.Context(), auth.HashToken(token)); err != nil {
			slog.Error("delete session", "err", err)
		}
	}
	s.clearSessionCookie(c)
	c.JSON(http.StatusOK, gin.H{"signedOut": true})
}

// ------------------------------------------------------------------ release notes

type notifyReq struct {
	NotifyReleases *bool `json:"notifyReleases" binding:"required"`
}

func (s *Server) handleNotifyReleases(c *gin.Context) {
	var req notifyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := s.store.SetNotifyReleases(c.Request.Context(), currentUser(c).ID, *req.NotifyReleases); err != nil {
		fail(c, err, "could not update that")
		return
	}
	c.JSON(http.StatusOK, gin.H{"notifyReleases": *req.NotifyReleases})
}

// ------------------------------------------------------------------ cookie

// setSessionCookie issues the one cookie Astralis uses.
//
//	HttpOnly   no script can read it, including one injected by an XSS bug
//	Secure     HTTPS only (off on localhost, which has no TLS)
//	SameSite   Lax: never sent on a cross-site POST, which is the CSRF defence
//	           for every state-changing endpoint here
//	Path=/     the app and the API share one origin behind Caddy
func (s *Server) setSessionCookie(c *gin.Context, token string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: auth.CookieName, Value: token, Path: "/",
		MaxAge:   int(auth.SessionTTL / time.Second),
		HttpOnly: true, Secure: s.cfg.Production(), SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: auth.CookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: s.cfg.Production(), SameSite: http.SameSiteLaxMode,
	})
}

// authRateLimit throttles the unauthenticated endpoints by client IP. There is
// no user yet to key on — which is exactly when abuse is cheapest.
func (s *Server) authRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.authLimiter.allow("ip:" + c.ClientIP()) {
			c.Header("Retry-After", "60")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":   "rate_limited",
				"message": "Too many attempts from this connection. Wait a minute and try again.",
			})
			return
		}
		c.Next()
	}
}
