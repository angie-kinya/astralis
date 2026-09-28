package api

import (
	"log/slog"
	"net/http"
	"strings"

	"astralis/gateway/internal/safety"
	"astralis/gateway/internal/tasks"

	"github.com/gin-gonic/gin"
)

// ConsentVersion for whenever the participation terms change
const ConsentVersion = "2026-09-v1"

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
		"consented": u.HasConsented(), "consentVersion": ConsentVersion,
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

// persist -> safety check -> EITHER fixed response OR generate -> persist reply
func (s *Server) handleSendMessage(c *gin.Context) {
	if !s.ownsSession(c) {
		return
	}
	var req sendMessageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid message"})
		return
	}

	ctx := c.Request.Context()
	sessionID := c.Param("id")
	user := currentUser(c)
	content := strings.TrimSpace(req.Content)

	// Safety runs before anything is generated, and before this message can influence retrieval or memory.
	verdict := s.filter.Check(ctx, content)

	userMsgID, err := s.store.AddMessage(ctx, sessionID, "user", content, verdict.IsCrisis(), verdict.Score)
	if err != nil {
		fail(c, err, "could not save message")
		return
	}

	if verdict.IsCrisis() {
		if err := s.store.FlagCrisis(ctx, userMsgID, verdict.MatchedRule, verdict.Score); err != nil {
			slog.Error("flag crisis", "err", err) // audit failure must not swallow the response
		}
		// The LLM is never called. Because the row is stored with crisis_flagged=true
		// RecentMessages excludes it and neither the prompt window nor the session summary will ever contain it.
		if _, err := s.store.AddMessage(ctx, sessionID, "assistant", safety.CrisisResponse, true, verdict.Score); err != nil {
			slog.Error("save crisis reply", "err", err)
		}
		c.JSON(http.StatusOK, gin.H{
			"reply": safety.CrisisResponse, "crisis": true, "sources": []any{},
		})
		return
	}

	answer, err := s.engine.Answer(ctx, user.ID, sessionID, content)
	if err != nil {
		fail(c, err, "could not generate a reply")
		return
	}

	replyID, err := s.store.AddMessage(ctx, sessionID, "assistant", answer.Text, false, 0)
	if err != nil {
		fail(c, err, "could not save reply")
		return
	}

	recID, err := s.store.SaveRecommendation(ctx, sessionID, replyID, answer.ChunkIDs)
	if err != nil {
		slog.Error("save recommendation", "err", err)
	}

	_ = s.store.TouchSession(ctx, sessionID)
	// The first user message doubles as the sidebar title.
	if n, _ := s.store.CountMessages(ctx, sessionID); n <= 2 {
		_ = s.store.SetSessionTitleIfEmpty(ctx, sessionID, truncate(content, 60))
	}

	c.JSON(http.StatusOK, gin.H{
		"reply":            answer.Text,
		"crisis":           false,
		"recommendationId": recID,
		"sources":          answer.Sources,
	})
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
