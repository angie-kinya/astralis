package api

import (
	"errors"
	"net/http"

	"astralis/gateway/internal/store"

	"github.com/gin-gonic/gin"
)

// handleListFacts is "What Astralis remembers" in settings: every fact the
// agent may recall, exactly as it is stored. nothing is hidden from its owner
func (s *Server) handleListFacts(c *gin.Context) {
	facts, err := s.store.ListFacts(c.Request.Context(), currentUser(c).ID)
	if err != nil {
		fail(c, err, "could not load what Astralis remembers")
		return
	}
	if facts == nil {
		facts = []store.Fact{} // [] not null, so the client never special-cases empty
	}
	c.JSON(http.StatusOK, gin.H{"facts": facts})
}

// handleDeleteFact forgets one fact. a hard delete: a later conversation may
// teach it again, but nothing of this row survives
func (s *Server) handleDeleteFact(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	err := s.store.DeleteFact(c.Request.Context(), currentUser(c).ID, id)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if err != nil {
		fail(c, err, "could not forget that")
		return
	}
	c.Status(http.StatusNoContent)
}

// handleLastTime feeds the quiet "last time" line at the top of a new chat.
// null when there is nothing recent enough to be worth saying
func (s *Server) handleLastTime(c *gin.Context) {
	f, err := s.store.LastTime(c.Request.Context(), currentUser(c).ID)
	if err != nil {
		fail(c, err, "could not load your last conversation")
		return
	}
	c.JSON(http.StatusOK, gin.H{"fact": f})
}
