package api

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleLatestReading returns recent headlines from the fixed feed allowlist.
// It deliberately takes nothing from the request and logs nothing about the
// caller: the list is the same for everyone, and which articles a person
// opens is between them and the publisher.
func (s *Server) handleLatestReading(c *gin.Context) {
	items, err := s.reading.Latest()
	if err != nil {
		slog.Warn("reading feeds", "err", err)
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "feeds_unavailable",
			"message": "The latest headlines could not be loaded just now.",
		})
		return
	}
	c.Header("Cache-Control", "private, max-age=300")
	c.JSON(http.StatusOK, gin.H{"items": items})
}
