package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"astralis/gateway/internal/store"

	"github.com/gin-gonic/gin"
)

// isAdmin compares the SHA-256 of the signed in email against the configured
// digests. The comparison is constant time: an admin check is exactly the
// kind of code where timing should reveal nothing.
func isAdmin(email string, hashes []string) bool {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email))))
	mine := hex.EncodeToString(sum[:])
	ok := false
	for _, h := range hashes {
		if subtle.ConstantTimeCompare([]byte(mine), []byte(h)) == 1 {
			ok = true
		}
	}
	return ok
}

// requireAdmin answers 404, not 403, to everyone else. A 403 would confirm
// that an admin area exists and invite someone to look for a way in.
func (s *Server) requireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isAdmin(currentUser(c).Email, s.cfg.AdminEmailHashes) {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.Next()
	}
}

// GET /api/v1/admin/overview?hours=24
func (s *Server) handleAdminOverview(c *gin.Context) {
	hours := boundedInt(c.Query("hours"), 24, 1, 24*30)
	ctx := c.Request.Context()
	o, err := s.store.AgentOverview(ctx, time.Duration(hours)*time.Hour)
	if err != nil {
		fail(c, err, "could not load overview")
		return
	}
	// circle reports sit beside crisis data: both are queues a person must review.
	// not windowed, an unreviewed report stays on screen until someone looks at it
	reports, err := s.store.UnreviewedReports(ctx)
	if err != nil {
		fail(c, err, "could not load overview")
		return
	}
	c.JSON(http.StatusOK, struct {
		store.Overview
		UnreviewedReports int `json:"unreviewedReports"`
	}{o, reports})
}

// GET /api/v1/admin/runs?limit=50
func (s *Server) handleAdminRuns(c *gin.Context) {
	runs, err := s.store.RecentRuns(c.Request.Context(), boundedInt(c.Query("limit"), 50, 1, 500))
	if err != nil {
		fail(c, err, "could not load runs")
		return
	}
	c.JSON(http.StatusOK, gin.H{"runs": runs})
}

// boundedInt parses a query value, falling back to def and clamping to
// [lo, hi], so no query string can ask the database for a million rows.
func boundedInt(raw string, def, lo, hi int) int {
	n, err := strconv.Atoi(raw)
	if err != nil {
		n = def
	}
	return max(lo, min(hi, n))
}
