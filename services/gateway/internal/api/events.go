package api

import (
	"io"
	"net/http"
	"regexp"
	"time"

	"astralis/gateway/internal/realtime"

	"github.com/gin-gonic/gin"
)

// handleEvents is GET /api/v1/events, one server-sent event stream per tab.
// the session cookie is same-origin, so EventSource authenticates with no extra work.
//
// http.Server has no WriteTimeout on purpose (cmd/api): one would cut every
// long-lived stream. Caddy must not buffer either: flush_interval -1 in deploy/Caddyfile
func (s *Server) handleEvents(c *gin.Context) {
	ch, unsubscribe := s.hub.Subscribe(currentUser(c).ID)
	defer unsubscribe()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	// a first byte now, so the browser marks the stream open instead of waiting for the first event
	_, _ = io.WriteString(c.Writer, ": open\n\n")
	c.Writer.Flush()

	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()
	c.Stream(func(w io.Writer) bool {
		select {
		case <-c.Request.Context().Done():
			return false
		case <-s.hub.Done(): // shutting down; EventSource reconnects to the new process
			return false
		case e, ok := <-ch:
			if !ok {
				return false
			}
			c.SSEvent(e.Type, e.Data)
		case <-heartbeat.C:
			_, _ = io.WriteString(w, ": ping\n\n") // keeps proxies from closing idle streams
		}
		return true
	})
}

// publish fans one event out to each user. nil-safe through the hub
func (s *Server) publish(e realtime.Event, userIDs ...string) {
	for _, id := range userIDs {
		s.hub.Publish(id, e)
	}
}

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// idParam reads a uuid path parameter. a malformed id is a 404, not a database
// error surfacing as a 500
func idParam(c *gin.Context, name string) (string, bool) {
	v := c.Param(name)
	if !uuidRe.MatchString(v) {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not found"})
		return "", false
	}
	return v, true
}
