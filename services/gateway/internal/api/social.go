package api

import (
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"astralis/gateway/internal/safety"
	"astralis/gateway/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// handleRe deliberately excludes '@' and '.' so a handle can never be mistaken
// for — or quietly become — an email address. 3-24 chars keeps it readable in
// a sidebar without truncation.
var handleRe = regexp.MustCompile(`^[a-zA-Z0-9_\- ]{3,24}$`)

type handleReq struct {
	Handle string `json:"handle" binding:"required"`
}

// handleSetHandle completes onboarding. The name a person picks here is the
// only identity other users ever see, and it is the name Astralis uses.
func (s *Server) handleSetHandle(c *gin.Context) {
	var req handleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid handle"})
		return
	}
	h := strings.TrimSpace(req.Handle)
	if !handleRe.MatchString(h) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_handle",
			"message": "3–24 characters: letters, numbers, spaces, hyphens or underscores.",
		})
		return
	}

	ctx := c.Request.Context()
	if taken, err := s.store.HandleTaken(ctx, h); err == nil && taken {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "handle_taken",
			"message": "Someone already goes by that here. Try another.",
		})
		return
	}
	if err := s.store.SetHandle(ctx, currentUser(c).ID, h); err != nil {
		fail(c, err, "could not save that name")
		return
	}
	c.JSON(http.StatusOK, gin.H{"handle": h})
}

type optInReq struct {
	// Pointer so that `false` is distinguishable from "field absent".
	PeerOptIn *bool `json:"peerOptIn" binding:"required"`
}

func (s *Server) handlePeerOptIn(c *gin.Context) {
	var req optInReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := s.store.SetPeerOptIn(c.Request.Context(), currentUser(c).ID, *req.PeerOptIn); err != nil {
		fail(c, err, "could not update your circle setting")
		return
	}
	c.JSON(http.StatusOK, gin.H{"peerOptIn": *req.PeerOptIn})
}

// ------------------------------------------------------------------ peers

func (s *Server) handleListPeers(c *gin.Context) {
	peers, err := s.store.ListPeers(c.Request.Context(), currentUser(c).ID)
	if err != nil {
		fail(c, err, "could not load your circle")
		return
	}
	c.JSON(http.StatusOK, gin.H{"peers": peers})
}

func (s *Server) handleGetConversation(c *gin.Context) {
	ctx := c.Request.Context()
	me := currentUser(c).ID
	peerID := c.Param("id")

	msgs, err := s.store.Conversation(ctx, me, peerID, 200)
	if err != nil {
		fail(c, err, "could not load that conversation")
		return
	}
	// Opening a thread marks it read. Best-effort: a failed read receipt is
	// not worth failing the request the person actually made.
	if err := s.store.MarkConversationRead(ctx, me, peerID); err != nil {
		slog.Error("mark read", "err", err)
	}
	c.JSON(http.StatusOK, gin.H{"messages": msgs})
}

type peerMsgReq struct {
	Body string `json:"body" binding:"required,min=1,max=2000"`
}

// handleSendPeerMessage runs the same safety filter as the agent path.
//
// This matters more here, not less: a person in crisis writing to another
// beta user is reaching someone with no training and no obligation to answer.
// A flagged message is stored for audit, never delivered, and the sender gets
// the same fixed resource card the agent would have given them.
func (s *Server) handleSendPeerMessage(c *gin.Context) {
	var req peerMsgReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid message"})
		return
	}

	ctx := c.Request.Context()
	me := currentUser(c).ID
	peerID := c.Param("id")
	body := strings.TrimSpace(req.Body)

	verdict := s.filter.Check(ctx, body)

	id, err := s.store.SendPeerMessage(ctx, me, peerID, body, verdict.IsCrisis())
	if errors.Is(err, store.ErrPeerUnavailable) {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "peer_unavailable",
			"message": "That person is no longer accepting messages.",
		})
		return
	}
	if err != nil {
		fail(c, err, "could not send that message")
		return
	}

	if verdict.IsCrisis() {
		if err := s.store.FlagPeerCrisis(ctx, id, verdict.MatchedRule, verdict.Score); err != nil {
			slog.Error("flag peer crisis", "err", err)
		}
		c.JSON(http.StatusOK, gin.H{
			"delivered": false,
			"crisis":    true,
			"reply":     safety.CrisisResponse,
		})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id, "delivered": true, "crisis": false})
}

func (s *Server) handleUnread(c *gin.Context) {
	n, err := s.store.UnreadTotal(c.Request.Context(), currentUser(c).ID)
	if err != nil {
		fail(c, err, "could not check messages")
		return
	}
	c.JSON(http.StatusOK, gin.H{"unread": n})
}

// ------------------------------------------------------------------ notes

func (s *Server) handleListNotes(c *gin.Context) {
	notes, err := s.store.ListNotes(c.Request.Context(), currentUser(c).ID)
	if err != nil {
		fail(c, err, "could not load your notes")
		return
	}
	c.JSON(http.StatusOK, gin.H{"notes": notes})
}

func (s *Server) handleCreateNote(c *gin.Context) {
	n, err := s.store.CreateNote(c.Request.Context(), currentUser(c).ID)
	if err != nil {
		fail(c, err, "could not create a note")
		return
	}
	c.JSON(http.StatusCreated, n)
}

type noteReq struct {
	Title string `json:"title" binding:"max=120"`
	Body  string `json:"body" binding:"max=20000"`
}

func (s *Server) handleSaveNote(c *gin.Context) {
	var req noteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid note"})
		return
	}
	err := s.store.SaveNote(c.Request.Context(), currentUser(c).ID,
		c.Param("id"), strings.TrimSpace(req.Title), req.Body)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "note not found"})
		return
	}
	if err != nil {
		fail(c, err, "could not save that note")
		return
	}
	c.JSON(http.StatusOK, gin.H{"saved": true})
}

func (s *Server) handleDeleteNote(c *gin.Context) {
	if err := s.store.DeleteNote(c.Request.Context(), currentUser(c).ID, c.Param("id")); err != nil {
		fail(c, err, "could not delete that note")
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}
