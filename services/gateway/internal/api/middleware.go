package api

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"astralis/gateway/internal/auth"
	"astralis/gateway/internal/store"

	"github.com/gin-gonic/gin"
)

const userKey = "astralis_user"

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next() // everything after this line runs on the way back out
		slog.Info("request",
			"method", c.Request.Method,
			"path", c.FullPath(), // the route pattern, not the URL: no session ids
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}
}

// authenticate resolves the session cookie to a user, or stops the request.
//
// This replaces two things the GitHub era needed: a JWT check and an upsert
// that created users on first sight. Users are now created in exactly one
// place — after an email has been proven in handleAuthVerify — so there is no
// path by which a request can conjure an account into existence. (The old
// upsert had one: a still-valid token presented after account deletion would
// quietly recreate the deleted user.)
func (s *Server) authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(auth.CookieName)
		if err != nil || token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "signed_out"})
			return
		}
		u, err := s.store.UserBySession(c.Request.Context(), auth.HashToken(token))
		if errors.Is(err, store.ErrNoSession) {
			// Clear the dead cookie so the browser stops presenting it, and the
			// landing page stops treating this visitor as signed in.
			s.clearSessionCookie(c)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "signed_out"})
			return
		}
		if err != nil {
			slog.Error("resolve session", "err", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "could not check your session"})
			return
		}
		c.Set(userKey, u)
		c.Next()
	}
}

// requireConsent enforces the rule at the routing layer rather than trusting every handler to remember it.
func (s *Server) requireConsent() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !currentUser(c).HasConsented() {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "consent_required",
				"message": "You must accept the participation terms before using Astralis.",
			})
			return
		}
		c.Next()
	}
}

func currentUser(c *gin.Context) store.User {
	return c.MustGet(userKey).(store.User)
}
