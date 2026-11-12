package api

import (
	"crypto/rand"
	"errors"
	"fmt"
	mrand "math/rand/v2"
	"net/http"
	"time"

	"astralis/gateway/internal/game"
	"astralis/gateway/internal/realtime"
	"astralis/gateway/internal/store"

	"github.com/gin-gonic/gin"
)

// constellations over http. the server is the only referee: a client sends an
// edge and the version it saw, never a board. every response and every event
// is shaped for the person receiving it, so "me", "myTurn" and the score read
// from their side of the table

// ------------------------------------------------------------------ shapes

type inviteJSON struct {
	ID         string    `json:"id"`
	PeerID     string    `json:"peerId"`
	PeerHandle string    `json:"peerHandle"`
	Mine       bool      `json:"mine"` // I sent it
	Status     string    `json:"status"`
	GameID     *string   `json:"gameId"`
	CreatedAt  time.Time `json:"createdAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

func inviteView(i store.InviteRow, viewer string) inviteJSON {
	v := inviteJSON{ID: i.ID, Status: i.Status, GameID: i.GameID, CreatedAt: i.CreatedAt, ExpiresAt: i.ExpiresAt}
	if i.FromID == viewer {
		v.Mine, v.PeerID, v.PeerHandle = true, i.ToID, i.ToHandle
	} else {
		v.PeerID, v.PeerHandle = i.FromID, i.FromHandle
	}
	return v
}

type scoreJSON struct {
	Me   int `json:"me"`
	Them int `json:"them"`
}

type gameJSON struct {
	ID         string           `json:"id"`
	Mode       string           `json:"mode"`
	PeerID     *string          `json:"peerId"`
	PeerHandle *string          `json:"peerHandle"`
	Me         int8             `json:"me"`
	Drawn      [game.Edges]bool `json:"drawn"`
	DrawnBy    [game.Edges]int8 `json:"drawnBy"` // absolute player, 0 undrawn; not flipped per receiver
	Owner      [game.Boxes]int8 `json:"owner"`
	ToMove     int8             `json:"toMove"`
	MyTurn     bool             `json:"myTurn"`
	Score      scoreJSON        `json:"score"`
	Status     string           `json:"status"`
	Result     *string          `json:"result"`
	LastEdge   *int             `json:"lastEdge"`
	BotEdges   []int            `json:"botEdges"`
	Version    int              `json:"version"`
	UpdatedAt  time.Time        `json:"updatedAt"`
}

// gameView shapes a game for one receiver. in solo the one person holds both
// colours, so me is 1, it is always their turn while the game runs, and there
// is no result to frame: the score speaks for itself
func gameView(g store.GameRow, viewer string, botEdges []int) gameJSON {
	st := g.State
	v := gameJSON{
		ID: g.ID, Mode: g.Mode, Me: 1,
		Drawn: st.Drawn, DrawnBy: st.DrawnBy, Owner: st.Owner, ToMove: st.ToMove,
		Status: g.Status, BotEdges: botEdges, Version: g.Version, UpdatedAt: g.UpdatedAt,
	}
	if v.BotEdges == nil {
		v.BotEdges = []int{}
	}
	if st.LastEdge >= 0 {
		e := st.LastEdge
		v.LastEdge = &e
	}
	if g.Mode == "peer" {
		if g.PlayerB != nil && *g.PlayerB == viewer {
			v.Me = 2
			v.PeerID, v.PeerHandle = &g.PlayerA, &g.HandleA
		} else {
			v.PeerID, v.PeerHandle = g.PlayerB, g.HandleB
		}
	}
	a, b := st.Score()
	v.Score = scoreJSON{Me: a, Them: b}
	if v.Me == 2 {
		v.Score = scoreJSON{Me: b, Them: a}
	}
	active := g.Status == "active"
	v.MyTurn = active && (g.Mode == "solo" || st.ToMove == v.Me)
	if g.Status == "finished" && g.Mode != "solo" {
		r := "lost"
		if st.Winner() == v.Me {
			r = "won"
		}
		v.Result = &r
	}
	return v
}

// ------------------------------------------------------------------ rules glue

var errNotYourTurn = errors.New("not your turn")

// play applies one person's edge, then, against astralis, the bot's replies
// until the person is to move again or the board is full. it returns the
// game to save and the bot's edges in the order they were drawn. pure, so the
// whole turn is decided before anything is written
func play(g store.GameRow, viewer string, edge int) (store.GameRow, []int, error) {
	var p int8
	switch g.Mode {
	case "solo":
		p = g.State.ToMove
	case "astralis":
		p = 1
	default:
		p = 1
		if g.PlayerB != nil && *g.PlayerB == viewer {
			p = 2
		}
	}
	if p != g.State.ToMove {
		return g, nil, errNotYourTurn
	}
	st, err := g.State.Apply(p, edge)
	if err != nil {
		return g, nil, err
	}
	g.State = st
	var bot []int
	if g.Mode == "astralis" {
		g.State, bot = botReplies(g.State, game.Seed(g.ID))
	}
	settle(&g)
	return g, bot, nil
}

// botReplies plays astralis (always player 2) while it holds the turn
func botReplies(st game.State, seed uint64) (game.State, []int) {
	bot := []int{}
	for !st.Over() && st.ToMove == 2 {
		e := game.BotMove(st, seed)
		next, err := st.Apply(2, e)
		if err != nil {
			// unreachable: BotMove only returns undrawn edges on an unfinished board
			break
		}
		st = next
		bot = append(bot, e)
	}
	return st, bot
}

// settle sets status and winner from the board. in astralis there is no
// second user to name, so winner is the person when they win and NULL when
// astralis does; solo never has one
func settle(g *store.GameRow) {
	if !g.State.Over() {
		return
	}
	g.Status = "finished"
	g.Winner = nil
	switch w := g.State.Winner(); {
	case g.Mode == "solo":
	case w == 1:
		g.Winner = &g.PlayerA
	case w == 2 && g.Mode == "peer":
		g.Winner = g.PlayerB
	}
}

// newGameID is a random v4 uuid. astralis and solo games are named before they
// are stored, because the bot's seed comes from the id
func newGameID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// ------------------------------------------------------------------ publishing

func (s *Server) publishInvite(t string, i store.InviteRow) {
	s.publish(realtime.Event{Type: t, Data: gin.H{"invite": inviteView(i, i.FromID)}}, i.FromID)
	s.publish(realtime.Event{Type: t, Data: gin.H{"invite": inviteView(i, i.ToID)}}, i.ToID)
}

// publishGame reaches every person in the game, each from their own side
func (s *Server) publishGame(g store.GameRow, botEdges []int) {
	s.publish(realtime.Event{Type: realtime.GameUpdate, Data: gin.H{"game": gameView(g, g.PlayerA, botEdges)}}, g.PlayerA)
	if g.PlayerB != nil {
		s.publish(realtime.Event{Type: realtime.GameUpdate, Data: gin.H{"game": gameView(g, *g.PlayerB, botEdges)}}, *g.PlayerB)
	}
}

// ------------------------------------------------------------------ invites

func (s *Server) handleSendInvite(c *gin.Context) {
	peerID, ok := idParam(c, "id")
	if !ok {
		return
	}
	me := currentUser(c).ID
	inv, err := s.store.CreateInvite(c.Request.Context(), me, peerID)
	switch {
	case errors.Is(err, store.ErrPeerUnavailable):
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "peer_unavailable",
			"message": "That person is no longer accepting messages.",
		})
		return
	case errors.Is(err, store.ErrInvitePending):
		c.JSON(http.StatusConflict, gin.H{
			"error":   "invite_pending",
			"message": "There is already an open invite between you two.",
		})
		return
	case errors.Is(err, store.ErrRateLimited):
		c.Header("Retry-After", "3600")
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":   "rate_limited",
			"message": "That is a few invites for one hour. Try again a little later.",
		})
		return
	case err != nil:
		fail(c, err, "could not send that invite")
		return
	}
	s.publishInvite(realtime.InviteNew, inv)
	c.JSON(http.StatusCreated, gin.H{"invite": inviteView(inv, me)})
}

func (s *Server) handleListInvites(c *gin.Context) {
	me := currentUser(c).ID
	rows, err := s.store.ListInvites(c.Request.Context(), me)
	if err != nil {
		fail(c, err, "could not load invites")
		return
	}
	out := make([]inviteJSON, 0, len(rows))
	for _, r := range rows {
		out = append(out, inviteView(r, me))
	}
	c.JSON(http.StatusOK, gin.H{"invites": out})
}

func inviteGone(c *gin.Context) {
	c.JSON(http.StatusConflict, gin.H{
		"error":   "invite_gone",
		"message": "That invite has already closed.",
	})
}

func (s *Server) handleAcceptInvite(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	me := currentUser(c).ID
	first := int8(mrand.IntN(2) + 1)
	inv, g, err := s.store.AcceptInvite(c.Request.Context(), me, id, first)
	switch {
	case errors.Is(err, store.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "invite not found"})
		return
	case errors.Is(err, store.ErrInviteGone):
		inviteGone(c)
		return
	case errors.Is(err, store.ErrPeerUnavailable):
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "peer_unavailable",
			"message": "That person is no longer accepting messages.",
		})
		return
	case err != nil:
		fail(c, err, "could not start that game")
		return
	}
	s.publishInvite(realtime.InviteUpdate, inv)
	s.publishGame(g, nil)
	c.JSON(http.StatusOK, gin.H{"game": gameView(g, me, nil)})
}

func (s *Server) handleDeclineInvite(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	inv, err := s.store.DeclineInvite(c.Request.Context(), currentUser(c).ID, id)
	switch {
	case errors.Is(err, store.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "invite not found"})
		return
	case errors.Is(err, store.ErrInviteGone):
		inviteGone(c)
		return
	case err != nil:
		fail(c, err, "could not close that invite")
		return
	}
	s.publishInvite(realtime.InviteUpdate, inv)
	c.Status(http.StatusNoContent)
}

// ------------------------------------------------------------------ games

type newGameReq struct {
	Mode string `json:"mode" binding:"required,oneof=astralis solo"`
}

// handleCreateGame starts a game with astralis or a solo board. neither needs
// the circle. against astralis the opener is random, as with people, and if
// astralis opens its first moves are already on the board when it is saved
func (s *Server) handleCreateGame(c *gin.Context) {
	var req newGameReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_mode"})
		return
	}
	id, err := newGameID()
	if err != nil {
		fail(c, err, "could not start a game")
		return
	}
	first := int8(1)
	if req.Mode == "astralis" {
		first = int8(mrand.IntN(2) + 1)
	}
	st := game.New(first)
	var bot []int
	if req.Mode == "astralis" {
		st, bot = botReplies(st, game.Seed(id))
	}
	me := currentUser(c).ID
	g, err := s.store.CreateGame(c.Request.Context(), id, me, req.Mode, st)
	if err != nil {
		fail(c, err, "could not start a game")
		return
	}
	s.publishGame(g, bot)
	c.JSON(http.StatusCreated, gin.H{"game": gameView(g, me, bot)})
}

func (s *Server) handleActiveGames(c *gin.Context) {
	me := currentUser(c).ID
	rows, err := s.store.ActiveGames(c.Request.Context(), me)
	if err != nil {
		fail(c, err, "could not load your games")
		return
	}
	out := make([]gameJSON, 0, len(rows))
	for _, g := range rows {
		out = append(out, gameView(g, me, nil))
	}
	c.JSON(http.StatusOK, gin.H{"games": out})
}

func (s *Server) handleGetGame(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	me := currentUser(c).ID
	g, err := s.store.GetGame(c.Request.Context(), me, id)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "game not found"})
		return
	}
	if err != nil {
		fail(c, err, "could not load that game")
		return
	}
	c.JSON(http.StatusOK, gin.H{"game": gameView(g, me, nil)})
}

type moveReq struct {
	// pointers so edge 0 and version 0 are not mistaken for missing fields
	Edge    *int `json:"edge" binding:"required"`
	Version *int `json:"version" binding:"required"`
}

func stale(c *gin.Context) {
	c.JSON(http.StatusConflict, gin.H{
		"error":   "stale",
		"message": "Someone moved first. Take another look at the board.",
	})
}

// handleMove checks the version before applying anything, so a move chosen
// against an old board is never played on a new one. the save repeats the
// check under the lock, which settles a race between two tabs
func (s *Server) handleMove(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req moveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "illegal_move"})
		return
	}
	ctx := c.Request.Context()
	me := currentUser(c).ID
	g, err := s.store.GetGame(ctx, me, id)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "game not found"})
		return
	}
	if err != nil {
		fail(c, err, "could not load that game")
		return
	}
	if g.Version != *req.Version || g.Status != "active" {
		stale(c)
		return
	}
	next, bot, err := play(g, me, *req.Edge)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "illegal_move"})
		return
	}
	saved, err := s.store.SaveMove(ctx, next, g.Version)
	if errors.Is(err, store.ErrStale) {
		stale(c)
		return
	}
	if err != nil {
		fail(c, err, "could not save that move")
		return
	}
	s.publishGame(saved, bot)
	c.JSON(http.StatusOK, gin.H{"game": gameView(saved, me, bot)})
}

// ------------------------------------------------------------------ leaderboard

func (s *Server) handleLeaderboard(c *gin.Context) {
	top, me, err := s.store.Leaderboard(c.Request.Context(), currentUser(c).ID)
	if err != nil {
		fail(c, err, "could not load the week")
		return
	}
	c.JSON(http.StatusOK, gin.H{"top": top, "me": me})
}

type leaderboardOptInReq struct {
	OptIn *bool `json:"optIn" binding:"required"`
}

func (s *Server) handleLeaderboardOptIn(c *gin.Context) {
	var req leaderboardOptInReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := s.store.SetLeaderboardOptIn(c.Request.Context(), currentUser(c).ID, *req.OptIn); err != nil {
		fail(c, err, "could not update that setting")
		return
	}
	c.Status(http.StatusNoContent)
}
