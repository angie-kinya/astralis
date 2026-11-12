package api

import (
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"astralis/gateway/internal/realtime"
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
			"message": "Use 3 to 24 characters. Letters, numbers, spaces, hyphens and underscores only.",
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
	peerID, ok := idParam(c, "id")
	if !ok {
		return
	}
	ctx := c.Request.Context()
	me := currentUser(c).ID

	msgs, err := s.store.Conversation(ctx, me, peerID, 200)
	if err != nil {
		fail(c, err, "could not load that conversation")
		return
	}
	// Opening a thread marks it read. Best-effort: a failed read receipt is
	// not worth failing the request the person actually made.
	if n, err := s.store.MarkConversationRead(ctx, me, peerID); err != nil {
		slog.Error("mark read", "err", err)
	} else if n > 0 {
		s.publish(realtime.Event{Type: realtime.MessageRead, Data: gin.H{"peerId": me}}, peerID)
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

	peerID, ok := idParam(c, "id")
	if !ok {
		return
	}
	ctx := c.Request.Context()
	me := currentUser(c).ID
	body := strings.TrimSpace(req.Body)

	verdict := s.filter.Check(ctx, body)

	msg, err := s.store.SendPeerMessage(ctx, me, peerID, body, verdict.IsCrisis())
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
		if err := s.store.FlagPeerCrisis(ctx, msg.ID, verdict.MatchedRule, verdict.Score); err != nil {
			slog.Error("flag peer crisis", "err", err)
		}
		c.JSON(http.StatusOK, gin.H{
			"delivered": false,
			"crisis":    true,
			"reply":     safety.CrisisResponse,
		})
		return
	}
	// delivered: the recipient's tabs and the sender's other tabs see it now,
	// each from their own side of the thread. flagged messages never get here
	s.publish(realtime.Event{Type: realtime.MessageNew, Data: gin.H{"peerId": peerID, "message": msg}}, me)
	theirs := msg
	theirs.Mine = false
	s.publish(realtime.Event{Type: realtime.MessageNew, Data: gin.H{"peerId": me, "message": theirs}}, peerID)

	c.JSON(http.StatusCreated, gin.H{"id": msg.ID, "delivered": true, "crisis": false, "message": msg})
}

// handleDeletePeerMessage tombstones one of the caller's own messages
func (s *Server) handleDeletePeerMessage(c *gin.Context) {
	if _, ok := idParam(c, "id"); !ok {
		return
	}
	msgID, ok := idParam(c, "msgId")
	if !ok {
		return
	}
	me := currentUser(c).ID
	// the store scopes by sender, so the :id in the path is never trusted for access
	peerID, err := s.store.DeletePeerMessage(c.Request.Context(), me, msgID)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "message not found"})
		return
	}
	if err != nil {
		fail(c, err, "could not delete that message")
		return
	}
	s.publish(realtime.Event{Type: realtime.MessageDeleted, Data: gin.H{"peerId": peerID, "messageId": msgID}}, me)
	s.publish(realtime.Event{Type: realtime.MessageDeleted, Data: gin.H{"peerId": me, "messageId": msgID}}, peerID)
	c.Status(http.StatusNoContent)
}

// handleBlockPeer is silent on purpose: nothing is published to the blocked
// person. to them it reads as someone who stopped accepting messages
func (s *Server) handleBlockPeer(c *gin.Context) {
	peerID, ok := idParam(c, "id")
	if !ok {
		return
	}
	err := s.store.BlockPeer(c.Request.Context(), currentUser(c).ID, peerID)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "person not found"})
		return
	}
	if err != nil {
		fail(c, err, "could not block that person")
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) handleUnblockPeer(c *gin.Context) {
	peerID, ok := idParam(c, "id")
	if !ok {
		return
	}
	if err := s.store.UnblockPeer(c.Request.Context(), currentUser(c).ID, peerID); err != nil {
		fail(c, err, "could not unblock that person")
		return
	}
	c.Status(http.StatusNoContent)
}

type reportReq struct {
	MessageID string `json:"messageId" binding:"omitempty,uuid"`
	Reason    string `json:"reason" binding:"required,oneof=harassment spam unsafe other"`
}

// handleReportPeer files a report for review. the snapshot is copied by the
// store from the message itself; the client only names which one
func (s *Server) handleReportPeer(c *gin.Context) {
	peerID, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req reportReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid report"})
		return
	}
	err := s.store.ReportPeer(c.Request.Context(), currentUser(c).ID, peerID, req.MessageID, req.Reason)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "nothing to report"})
		return
	}
	if err != nil {
		fail(c, err, "could not send that report")
		return
	}
	c.Status(http.StatusNoContent)
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
