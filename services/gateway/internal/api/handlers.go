package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"astralis/gateway/internal/agent"
	"astralis/gateway/internal/store"
	"astralis/gateway/internal/tasks"

	"github.com/gin-gonic/gin"
)

// ConsentVersion changes whenever the participation terms change. Everyone
// who accepted an older version is asked again before their next message.
// v2 adds the structured observations the agent records about each message.
const ConsentVersion = "2026-10-v2"

func (s *Server) handleMe(c *gin.Context) {
	u := currentUser(c)
	handle := ""
	if u.Handle != nil {
		handle = *u.Handle
	}
	c.JSON(http.StatusOK, gin.H{
		// email is returned to its owner and nobody else. No other endpoint
		// in this service ever serialises it.
		"id": u.ID, "email": u.Email,
		"consented": u.HasConsentedTo(ConsentVersion), "consentVersion": ConsentVersion,
		"notifyReleases": u.NotifyReleases,
		// handle drives onboarding: empty means Astralis has not yet asked
		// what to call this person.
		"handle": handle, "peerOptIn": u.PeerOptIn,
	})
}

func (s *Server) handleConsent(c *gin.Context) {
	if err := s.store.GrantConsent(c.Request.Context(), currentUser(c).ID, ConsentVersion); err != nil {
		fail(c, err, "could not record consent")
		return
	}
	c.JSON(http.StatusOK, gin.H{"consented": true, "version": ConsentVersion})
}

func (s *Server) handleDeleteMe(c *gin.Context) {
	if err := s.store.DeleteUser(c.Request.Context(), currentUser(c).ID); err != nil {
		fail(c, err, "could not delete account")
		return
	}
	// The cascade already removed every session row; clear the cookie too so
	// the browser is not left holding a token for an account that is gone.
	s.clearSessionCookie(c)
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (s *Server) handleListSessions(c *gin.Context) {
	list, err := s.store.ListSessions(c.Request.Context(), currentUser(c).ID)
	if err != nil {
		fail(c, err, "could not list sessions")
		return
	}
	c.JSON(http.StatusOK, gin.H{"sessions": list})
}

func (s *Server) handleCreateSession(c *gin.Context) {
	id, err := s.store.CreateSession(c.Request.Context(), currentUser(c).ID)
	if err != nil {
		fail(c, err, "could not create session")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func (s *Server) handleGetMessages(c *gin.Context) {
	if !s.ownsSession(c) {
		return
	}
	msgs, err := s.store.RecentMessages(c.Request.Context(), c.Param("id"), 100)
	if err != nil {
		fail(c, err, "could not load messages")
		return
	}
	c.JSON(http.StatusOK, gin.H{"messages": msgs})
}

type sendMessageReq struct {
	Content string `json:"content" binding:"required,min=1,max=4000"`
}

// handleSendMessage is deliberately thin. everything between receiving the
// text and returning the reply is the agent's job; see package agent
func (s *Server) handleSendMessage(c *gin.Context) {
	if !s.ownsSession(c) {
		return
	}
	var req sendMessageReq
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid message"})
		return
	}

	res, err := s.agent.Handle(c.Request.Context(), agent.Turn{
		UserID:    currentUser(c).ID,
		SessionID: c.Param("id"),
		Content:   strings.TrimSpace(req.Content),
	})
	if err != nil {
		fail(c, err, "could not generate a reply")
		return
	}

	body := gin.H{"reply": res.Reply, "crisis": res.Crisis, "sources": res.Sources}
	if res.RecommendationID != "" {
		body["recommendationId"] = res.RecommendationID
	}

	c.JSON(http.StatusOK, body)
}

// handleEndSession enqueues the summary job and returns straight away
func (s *Server) handleEndSession(c *gin.Context) {
	if !s.ownsSession(c) {
		return
	}
	// No queue configured (the free/single-VM deployment): nothing to do here.
	// cmd/summarise finds quiet sessions on its own schedule.
	if s.queue == nil {
		c.JSON(http.StatusAccepted, gin.H{"queued": false})
		return
	}
	task, err := tasks.NewSummariseTask(c.Param("id"), currentUser(c).ID)
	if err != nil {
		fail(c, err, "could not queue summary")
		return
	}
	if _, err := s.queue.Enqueue(task); err != nil {
		// A missing summary is a degraded experience not a failed request.
		slog.Error("enqueue summarise", "err", err)
	}
	c.JSON(http.StatusAccepted, gin.H{"queued": true})
}

type feedbackReq struct {
	RecommendationID string `json:"recommendationId" binding:"required,uuid"`
	Rating           int    `json:"rating" binding:"required,min=-1,max=1"`
	Comment          string `json:"comment" binding:"max=1000"`
}

func (s *Server) handleFeedback(c *gin.Context) {
	var req feedbackReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid feedback"})
		return
	}
	err := s.store.SaveFeedback(c.Request.Context(), req.RecommendationID,
		currentUser(c).ID, req.Rating, req.Comment)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "reply not found"})
		return
	}
	if err != nil {
		fail(c, err, "could not save feedback")
		return
	}
	c.JSON(http.StatusOK, gin.H{"saved": true})
}

// ownsSession blocks horizontal privilege escalation: a valid token for user A must not read user B's session by guessing a UUID
func (s *Server) ownsSession(c *gin.Context) bool {
	ok, err := s.store.SessionBelongsTo(c.Request.Context(), c.Param("id"), currentUser(c).ID)
	if err != nil || !ok {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return false
	}
	return true
}

// fail logs the real error and returns a generic one
func fail(c *gin.Context, err error, msg string) {
	slog.Error(msg, "err", err, "path", c.FullPath())
	c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
}

// Rune-aware: s[:n] on a string slices bytes, so a title cut mid-character produces invalid UTF-8. Any non-ASCII input finds this instantly.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
