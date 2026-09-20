package api

import (
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

// resolveUser maps the JWT subject to a row in the users table, creating it on first sight.
// Every handler downstream can then assume a real user exists which is why none of them contain a nil check.
func (s *Server) resolveUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := auth.From(c)
		u, err := s.store.UpsertUser(c.Request.Context(), claims.Subject, claims.Email, claims.Name)
		if err != nil {
			slog.Error("upsert user", "err", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "user lookup failed"})
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
