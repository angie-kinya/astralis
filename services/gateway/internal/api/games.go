package api

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"log/slog"
	"net/http"
	"time"

	"astralis/gateway/internal/game"
	"astralis/gateway/internal/game/constellations"
	"astralis/gateway/internal/game/registry"
	"astralis/gateway/internal/realtime"
	"astralis/gateway/internal/store"

	"github.com/gin-gonic/gin"
)

// games over http, every kind through one surface. the server is the only
// referee: a client sends a move and the version it saw, never a state, and
// the engine for the row's kind decides. every response and every event is
// shaped for the person receiving it, so "me", "myTurn" and the score read
// from their side of the table

// ------------------------------------------------------------------ shapes

type inviteJSON struct {
	ID         string       `json:"id"`
	PeerID     string       `json:"peerId"`
	PeerHandle string       `json:"peerHandle"`
	Mine       bool         `json:"mine"` // I sent it
	Status     string       `json:"status"`
	GameID     *string      `json:"gameId"`
	CreatedAt  time.Time    `json:"createdAt"`
	ExpiresAt  time.Time    `json:"expiresAt"`
	Kind       string       `json:"kind"`
	Options    game.Options `json:"options"`
}

func inviteView(i store.InviteRow, viewer string) inviteJSON {
	v := inviteJSON{ID: i.ID, Status: i.Status, GameID: i.GameID, CreatedAt: i.CreatedAt, ExpiresAt: i.ExpiresAt,
		Kind: i.Kind, Options: i.Options}
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
	ID         string       `json:"id"`
	Kind       string       `json:"kind"`
	Mode       string       `json:"mode"`
	Level      int          `json:"level"`
	Options    game.Options `json:"options"`
	PeerID     *string      `json:"peerId"`
	PeerHandle *string      `json:"peerHandle"`
	Me         int8         `json:"me"`
	MyTurn     bool         `json:"myTurn"`
	Status     string       `json:"status"`
	Result     *string      `json:"result"`
	Score      *scoreJSON   `json:"score"`
	View       any          `json:"view"`
	BotEdges   []int        `json:"botEdges"` // constellations vs astralis: edges astralis drew this request
	Version    int          `json:"version"`
	UpdatedAt  time.Time    `json:"updatedAt"`
}

// seatDoner is an optional engine method for games where both seats move at
// once (a words duel): it says whether one seat has nothing left to do, so
// myTurn can go false for the person waiting on the other
type seatDoner interface {
	SeatDone(s game.State, seat int8) bool
}

// seatOutcome is what a puzzle's view says about one seat. words keeps each
// seat's own progress there and not in Result (in a duel Result.Solved means
// either seat solved), so a view with these fields is the authority for that
// receiver. nil fields mean the view does not say
type seatOutcome struct {
	Over   *bool `json:"over"`
	Solved *bool `json:"solved"`
}

func outcomeOf(view any) seatOutcome {
	var o seatOutcome
	if b, err := json.Marshal(view); err == nil {
		_ = json.Unmarshal(b, &o) // a view that is not an object simply says nothing
	}
	return o
}

var errUnknownKind = errors.New("unknown game kind")

// lookup is registry.Get, a variable so tests can add engines of their own
var lookup = registry.Get

func engineFor(kind string) (game.Engine, error) {
	e, ok := lookup(kind)
	if !ok {
		return nil, fmt.Errorf("%w %q", errUnknownKind, kind)
	}
	return e, nil
}

// isBoard is true for games with two sides on one board, the kind astralis can
// play. solo there means one person holding both colours, so there is a score
// to show but nobody to frame a result for; solo anything else is a puzzle
func isBoard(e game.Engine) bool {
	_, ok := e.(game.Bot)
	return ok
}

// seatOf is the viewer's seat: 2 only for player_b in a peer game
func seatOf(g store.GameRow, viewer string) int8 {
	if g.Mode == "peer" && g.PlayerB != nil && *g.PlayerB == viewer {
		return 2
	}
	return 1
}

// gameView shapes a game for one receiver. the error is a row its engine
// cannot read (an unknown kind, a corrupt state)
func gameView(g store.GameRow, viewer string, botEdges []int) (gameJSON, error) {
	e, err := engineFor(g.Kind)
	if err != nil {
		return gameJSON{}, err
	}
	me := seatOf(g, viewer)
	view, err := e.View(g.State, me)
	if err != nil {
		return gameJSON{}, fmt.Errorf("game %s: %w", g.ID, err)
	}
	v := gameJSON{
		ID: g.ID, Kind: g.Kind, Mode: g.Mode, Level: g.Level, Options: g.Options, Me: me,
		Status: g.Status, View: view, BotEdges: botEdges, Version: g.Version, UpdatedAt: g.UpdatedAt,
	}
	if v.BotEdges == nil {
		v.BotEdges = []int{}
	}
	if g.Mode == "peer" {
		if me == 2 {
			v.PeerID, v.PeerHandle = &g.PlayerA, &g.HandleA
		} else {
			v.PeerID, v.PeerHandle = g.PlayerB, g.HandleB
		}
	}

	r := e.Result(g.State)
	puzzle := g.Mode == "solo" && !isBoard(e)
	if !puzzle {
		v.Score = &scoreJSON{Me: r.Score[me-1], Them: r.Score[2-me]}
	}
	var seat seatOutcome
	if !isBoard(e) {
		seat = outcomeOf(view)
	}
	solved := r.Solved
	if seat.Solved != nil {
		solved = *seat.Solved
	}
	if g.Status == "active" {
		switch tm := e.ToMove(g.State); {
		case seat.Over != nil:
			// a puzzle or duel: my turn for as long as my own board is open
			v.MyTurn = !*seat.Over
		case g.Mode == "solo", tm == me:
			v.MyTurn = true
		case tm == 0:
			d, ok := e.(seatDoner)
			v.MyTurn = !ok || !d.SeatDone(g.State, me)
		}
	}
	if g.Status == "finished" {
		var res string
		switch {
		case puzzle && solved:
			res = "solved"
		case puzzle:
			res = "unsolved"
		case g.Mode == "solo":
			// both colours were the same person: the score speaks for itself
		case r.Draw:
			res = "draw"
		case r.Winner == me:
			res = "won"
		default:
			res = "lost"
		}
		if res != "" {
			v.Result = &res
		}
	}
	return v, nil
}

// ------------------------------------------------------------------ rules glue

var errNotYourTurn = errors.New("not your turn")

// play applies one person's move, then, against astralis, the bot's replies
// until the person is to move again or the game is over. it returns the game
// to save, the bot's edges in the order they were drawn, and who moves next.
// pure, so the whole turn is decided before anything is written
func play(g store.GameRow, viewer string, move json.RawMessage) (store.GameRow, []int, int8, error) {
	e, err := engineFor(g.Kind)
	if err != nil {
		return g, nil, 0, err
	}
	tm := e.ToMove(g.State)
	seat := seatOf(g, viewer)
	if g.Mode == "solo" && tm != 0 {
		seat = tm
	}
	if tm != 0 && tm != seat {
		return g, nil, 0, errNotYourTurn
	}
	st, err := e.Apply(g.State, seat, move)
	if err != nil {
		return g, nil, 0, err
	}
	var bot []int
	if g.Mode == "astralis" {
		if st, bot, err = botReplies(e, st, seedOf(g.ID), g.Options); err != nil {
			return g, nil, 0, err
		}
	}
	g.State = st
	settle(e, &g)
	return g, bot, e.ToMove(g.State), nil
}

// botReplies plays astralis (always seat 2) while it holds the turn. the edges
// come back for the board to animate; a move without one is still played
func botReplies(e game.Engine, st game.State, seed uint64, o game.Options) (game.State, []int, error) {
	b, ok := e.(game.Bot)
	if !ok {
		return st, nil, fmt.Errorf("%s has no astralis opponent", e.Kind())
	}
	edges := []int{}
	// a bound, not a rule: an engine bug must not spin a request forever
	for range 1000 {
		if e.ToMove(st) != 2 {
			return st, edges, nil
		}
		m, err := b.BotMove(st, seed, o)
		if err != nil {
			return st, nil, err
		}
		if st, err = e.Apply(st, 2, m); err != nil {
			return st, nil, fmt.Errorf("bot move %s: %w", m, err)
		}
		var mv struct {
			Edge *int `json:"edge"`
		}
		if json.Unmarshal(m, &mv) == nil && mv.Edge != nil {
			edges = append(edges, *mv.Edge)
		}
	}
	return st, nil, errors.New("bot did not hand the turn back")
}

// settle sets status and winner from the engine's result. in astralis there is
// no second user to name, so winner is the person when they win and NULL when
// astralis does; solo and draws never have one, which is also how the
// leaderboard tells a draw
func settle(e game.Engine, g *store.GameRow) {
	r := e.Result(g.State)
	if !r.Over {
		return
	}
	g.Status = "finished"
	g.Winner = nil
	switch {
	case g.Mode == "solo", r.Draw:
	case r.Winner == 1:
		g.Winner = &g.PlayerA
	case r.Winner == 2 && g.Mode == "peer":
		g.Winner = g.PlayerB
	}
}

// seedOf turns a game id into its seed, so a given game deals and replays the
// same way and nothing about it depends on the clock. fnv as in 0008, so a
// game started before levels keeps the bot it began with
func seedOf(gameID string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(gameID))
	return h.Sum64()
}

// newGameID is a random v4 uuid. games are named before they are stored,
// because the seed comes from the id
func newGameID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// ------------------------------------------------------------------ levels

// defaultLevel is the 4x4 board every game was played on before levels, and
// what a client that sends no level still gets when it is open to them
const defaultLevel = 2

// unlockedLevel is the highest constellations level a person may start
// against astralis or alone: one past the highest they finished that way, and
// never below a level they have finished in any mode, so someone who has only
// played people keeps the board they know
func unlockedLevel(botOrSolo, anyMode int) int {
	return min(max(botOrSolo+1, anyMode, 1), constellations.MaxLevel)
}

// peerLevelCap is the highest level two people can play together: whatever
// both have unlocked, and the first two levels always, because a peer game on
// the 4x4 board needed nothing unlocked before levels existed
func peerLevelCap(a, b int) int { return max(min(a, b), defaultLevel) }

func (s *Server) unlocked(c *gin.Context, userID string) (int, error) {
	botOrSolo, anyMode, err := s.store.Progress(c.Request.Context(), userID)
	if err != nil {
		return 0, err
	}
	return unlockedLevel(botOrSolo, anyMode), nil
}

func levelLocked(c *gin.Context) {
	c.JSON(http.StatusForbidden, gin.H{
		"error":   "level_locked",
		"message": "Finish the level before to open this one.",
	})
}

// invalidOptions passes an engine's New error through: engines word them for
// people ("that topic isn't one we have"), and nothing in them is private
func invalidOptions(c *gin.Context, err error) {
	msg := "Those options don't make a game."
	if err != nil {
		msg = err.Error()
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_options", "message": msg})
}

func illegalMove(c *gin.Context) {
	c.JSON(http.StatusBadRequest, gin.H{"error": "illegal_move", "message": "That move doesn't fit here."})
}

// ------------------------------------------------------------------ publishing

func (s *Server) publishInvite(t string, i store.InviteRow) {
	s.publish(realtime.Event{Type: t, Data: gin.H{"invite": inviteView(i, i.FromID)}}, i.FromID)
	s.publish(realtime.Event{Type: t, Data: gin.H{"invite": inviteView(i, i.ToID)}}, i.ToID)
}

// publishGame reaches every person in the game, each from their own side
func (s *Server) publishGame(g store.GameRow, botEdges []int) {
	players := []string{g.PlayerA}
	if g.PlayerB != nil {
		players = append(players, *g.PlayerB)
	}
	for _, p := range players {
		v, err := gameView(g, p, botEdges)
		if err != nil {
			slog.Error("shape game event", "err", err)
			return
		}
		s.publish(realtime.Event{Type: realtime.GameUpdate, Data: gin.H{"game": v}}, p)
	}
}

// ------------------------------------------------------------------ catalogue

func (s *Server) handleCatalog(c *gin.Context) {
	out := gin.H{}
	for _, k := range registry.Kinds() {
		e, _ := registry.Get(k)
		if cat, ok := e.(game.Cataloguer); ok {
			out[k] = cat.Catalog()
		}
	}
	c.JSON(http.StatusOK, out)
}

func (s *Server) handleProgress(c *gin.Context) {
	n, err := s.unlocked(c, currentUser(c).ID)
	if err != nil {
		fail(c, err, "could not load your progress")
		return
	}
	c.JSON(http.StatusOK, gin.H{"constellations": gin.H{"unlocked": n}})
}

// ------------------------------------------------------------------ invites

type optionsReq struct {
	Level int    `json:"level"`
	Topic string `json:"topic"`
	Mode  string `json:"mode"`
	Daily bool   `json:"daily"`
}

func (o optionsReq) options() game.Options {
	return game.Options{Level: o.Level, Topic: o.Topic, Mode: o.Mode}
}

// canonical gives every way of asking for the same variant one spelling, so
// "" and "classic" cannot seed two different dailies for the same words puzzle
func canonical(kind string, o game.Options) game.Options {
	if kind == "words" && o.Mode == "" {
		o.Mode = "classic"
	}
	return o
}

// inviteReq may be absent altogether: the 0008 client sends no body and gets
// constellations on the board it knows
type inviteReq struct {
	Kind    string     `json:"kind"`
	Options optionsReq `json:"options"`
}

func (s *Server) handleSendInvite(c *gin.Context) {
	peerID, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req inviteReq
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		invalidOptions(c, nil)
		return
	}
	if req.Kind == "" {
		req.Kind = constellations.Kind
	}
	e, err := engineFor(req.Kind)
	if err != nil || e.Seats() != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown_kind"})
		return
	}
	me := currentUser(c).ID
	o := canonical(req.Kind, req.Options.options())
	if req.Kind == constellations.Kind {
		if o.Level == 0 {
			o.Level = defaultLevel
		}
		open, err := s.peerLevelOpen(c, me, peerID, o.Level)
		if err != nil {
			fail(c, err, "could not send that invite")
			return
		}
		if !open {
			levelLocked(c)
			return
		}
	}
	// dealing a throwaway game is the one check every engine already has
	if _, err := e.New(0, o, 2); err != nil {
		invalidOptions(c, err)
		return
	}

	inv, err := s.store.CreateInvite(c.Request.Context(), me, peerID, req.Kind, o)
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

var errLevelLocked = errors.New("level not open to both")

// peerLevelOpen is the invite rule, checked when it is sent and again when it
// is accepted: a level both people have open (levels 1 and 2 always are)
func (s *Server) peerLevelOpen(c *gin.Context, a, b string, level int) (bool, error) {
	la, err := s.unlocked(c, a)
	if err != nil {
		return false, err
	}
	lb, err := s.unlocked(c, b)
	if err != nil {
		return false, err
	}
	return level <= peerLevelCap(la, lb), nil
}

// startPeer deals the game an invite offers, re-checking the level: either
// person's progress can only grow, but an invite outlives nothing else that
// keeps the rule honest. the id is random, so the seed's low bit (who opens)
// is a fair coin, as it always was
func (s *Server) startPeer(c *gin.Context) store.StartFunc {
	return func(inv store.InviteRow) (store.GameRow, int8, error) {
		if inv.Kind == constellations.Kind {
			ok, err := s.peerLevelOpen(c, inv.FromID, inv.ToID, inv.Options.Level)
			if err != nil {
				return store.GameRow{}, 0, err
			}
			if !ok {
				return store.GameRow{}, 0, errLevelLocked
			}
		}
		return startPeer(inv)
	}
}

func startPeer(inv store.InviteRow) (store.GameRow, int8, error) {
	e, err := engineFor(inv.Kind)
	if err != nil {
		return store.GameRow{}, 0, err
	}
	id, err := newGameID()
	if err != nil {
		return store.GameRow{}, 0, err
	}
	st, err := e.New(seedOf(id), inv.Options, 2)
	if err != nil {
		return store.GameRow{}, 0, err
	}
	return store.GameRow{ID: id, Kind: inv.Kind, Level: inv.Options.Level, Options: inv.Options, State: st},
		e.ToMove(st), nil
}

func (s *Server) handleAcceptInvite(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	me := currentUser(c).ID
	inv, g, err := s.store.AcceptInvite(c.Request.Context(), me, id, s.startPeer(c))
	switch {
	case errors.Is(err, errLevelLocked):
		levelLocked(c)
		return
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
	s.respondGame(c, http.StatusOK, g, me, nil)
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

func (s *Server) respondGame(c *gin.Context, status int, g store.GameRow, viewer string, botEdges []int) {
	v, err := gameView(g, viewer, botEdges)
	if err != nil {
		fail(c, err, "could not load that game")
		return
	}
	c.JSON(status, gin.H{"game": v})
}

// newGameReq: kind defaults to constellations, so the 0008 client's {mode}
// still starts the game it expects
type newGameReq struct {
	Kind    string     `json:"kind"`
	Mode    string     `json:"mode" binding:"required,oneof=astralis solo"`
	Options optionsReq `json:"options"`
}

// handleCreateGame starts a game with astralis, a solo board or a puzzle.
// none needs the circle. against astralis the opener comes from the seed, as
// with people, and if astralis opens its first moves are already on the board
// when it is saved. a daily puzzle is get-or-create: 201 when dealt, 200 when
// today's was already there
func (s *Server) handleCreateGame(c *gin.Context) {
	var req newGameReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_mode"})
		return
	}
	if req.Kind == "" {
		req.Kind = constellations.Kind
	}
	e, err := engineFor(req.Kind)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown_kind"})
		return
	}
	board := isBoard(e)
	if req.Mode == "astralis" && !board {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_mode"})
		return
	}
	// a board game has no daily: there is nothing to be the same for everyone
	if req.Options.Daily && board {
		invalidOptions(c, errors.New("Board games have no daily puzzle."))
		return
	}
	me := currentUser(c).ID
	o := canonical(req.Kind, req.Options.options())
	if req.Kind == constellations.Kind {
		open, err := s.unlocked(c, me)
		if err != nil {
			fail(c, err, "could not start a game")
			return
		}
		if o.Level == 0 {
			o.Level = min(defaultLevel, open)
		}
		if o.Level > open {
			levelLocked(c)
			return
		}
	}

	id, err := newGameID()
	if err != nil {
		fail(c, err, "could not start a game")
		return
	}
	var seed uint64
	var daily *string
	if req.Options.Daily {
		o.Day = game.Today(time.Now())
		seed, daily = game.DailySeed(req.Kind, o), &o.Day
	} else {
		seed = seedOf(id)
		if req.Mode == "solo" {
			// seat 1 opens a solo board; for a puzzle this is just a seed
			seed &^= 1
		}
	}
	st, err := e.New(seed, o, 1)
	if err != nil {
		invalidOptions(c, err)
		return
	}
	var bot []int
	if req.Mode == "astralis" {
		if st, bot, err = botReplies(e, st, seedOf(id), o); err != nil {
			fail(c, err, "could not start a game")
			return
		}
	}
	row := store.GameRow{ID: id, Kind: req.Kind, Mode: req.Mode, Level: o.Level, Options: o,
		DailyKey: daily, PlayerA: me, State: st}
	g, created, err := s.store.CreateGame(c.Request.Context(), row, e.ToMove(st))
	if err != nil {
		fail(c, err, "could not start a game")
		return
	}
	if !created {
		s.respondGame(c, http.StatusOK, g, me, nil)
		return
	}
	s.publishGame(g, bot)
	s.respondGame(c, http.StatusCreated, g, me, bot)
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
		v, err := gameView(g, me, nil)
		if err != nil {
			// one unreadable row must not hide every other game
			slog.Error("shape active game", "err", err)
			continue
		}
		out = append(out, v)
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
	s.respondGame(c, http.StatusOK, g, me, nil)
}

type moveReq struct {
	Move json.RawMessage `json:"move"`
	// the 0008 client's shape, {edge, version}; a pointer so edge 0 counts
	Edge *int `json:"edge"`
	// a pointer so version 0 is not mistaken for a missing field
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
		illegalMove(c)
		return
	}
	if len(req.Move) == 0 && req.Edge != nil {
		req.Move, _ = json.Marshal(map[string]int{"edge": *req.Edge})
	}
	if len(req.Move) == 0 {
		illegalMove(c)
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
	next, bot, toMove, err := play(g, me, req.Move)
	if errors.Is(err, game.ErrIllegal) || errors.Is(err, errNotYourTurn) {
		illegalMove(c)
		return
	}
	if err != nil {
		fail(c, err, "could not play that move")
		return
	}
	saved, err := s.store.SaveMove(ctx, next, g.Version, toMove)
	if errors.Is(err, store.ErrStale) {
		stale(c)
		return
	}
	if err != nil {
		fail(c, err, "could not save that move")
		return
	}
	s.publishGame(saved, bot)
	s.respondGame(c, http.StatusOK, saved, me, bot)
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
