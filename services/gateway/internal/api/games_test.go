package api

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"astralis/gateway/internal/config"
	"astralis/gateway/internal/game"
	"astralis/gateway/internal/game/constellations"
	"astralis/gateway/internal/game/registry"
	"astralis/gateway/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const (
	alice = "aaaaaaaa-0000-4000-8000-000000000001"
	bruno = "bbbbbbbb-0000-4000-8000-000000000002"
)

func strp(s string) *string { return &s }

func newState(t *testing.T, level int, first int8) constellations.State {
	t.Helper()
	st, err := constellations.NewState(level, first)
	require.NoError(t, err)
	return st
}

func raw(t *testing.T, st constellations.State) game.State {
	t.Helper()
	b, err := json.Marshal(st)
	require.NoError(t, err)
	return b
}

func decodeState(t *testing.T, s game.State) constellations.State {
	t.Helper()
	var st constellations.State
	require.NoError(t, json.Unmarshal(s, &st))
	return st
}

func peerGame(t *testing.T, st constellations.State) store.GameRow {
	return store.GameRow{
		ID: "11111111-0000-4000-8000-000000000003", Kind: "constellations", Mode: "peer",
		Level: st.Level, Options: game.Options{Level: st.Level},
		PlayerA: alice, HandleA: "alice", PlayerB: strp(bruno), HandleB: strp("bruno"),
		State: raw(t, st), Status: "active", UpdatedAt: time.Unix(0, 0),
	}
}

func edge(e int) json.RawMessage {
	b, _ := json.Marshal(map[string]int{"edge": e})
	return b
}

// fill plays edges in order for whoever is to move
func fill(t *testing.T, st constellations.State, edges ...int) constellations.State {
	t.Helper()
	for _, e := range edges {
		var err error
		st, err = st.Apply(st.ToMove, e)
		require.NoError(t, err)
	}
	return st
}

func view(t *testing.T, g store.GameRow, viewer string) gameJSON {
	t.Helper()
	v, err := gameView(g, viewer, nil)
	require.NoError(t, err)
	return v
}

func TestInviteView(t *testing.T) {
	inv := store.InviteRow{ID: "i", FromID: alice, FromHandle: "alice", ToID: bruno, ToHandle: "bruno",
		Status: "pending", Kind: "words", Options: game.Options{Topic: "calm", Mode: "classic"}}
	a, b := inviteView(inv, alice), inviteView(inv, bruno)
	require.True(t, a.Mine)
	require.Equal(t, bruno, a.PeerID)
	require.Equal(t, "bruno", a.PeerHandle)
	require.False(t, b.Mine)
	require.Equal(t, alice, b.PeerID)
	require.Equal(t, "alice", b.PeerHandle)

	out, err := json.Marshal(a)
	require.NoError(t, err)
	require.Contains(t, string(out), `"gameId":null`)
	require.Contains(t, string(out), `"kind":"words","options":{"topic":"calm","mode":"classic"}`)
}

func TestGameViewPerReceiver(t *testing.T) {
	// player 2 closes box 0 with edge 13 and keeps the turn
	st := fill(t, newState(t, 2, 1), 0, 3, 12, 13)
	require.Equal(t, int8(2), st.Owner[0])
	g := peerGame(t, st)

	cases := []struct {
		viewer   string
		me       int8
		myTurn   bool
		score    scoreJSON
		peerName string
	}{
		{alice, 1, false, scoreJSON{Me: 0, Them: 1}, "bruno"},
		{bruno, 2, true, scoreJSON{Me: 1, Them: 0}, "alice"},
	}
	for _, tc := range cases {
		v := view(t, g, tc.viewer)
		require.Equal(t, tc.me, v.Me, tc.viewer)
		require.Equal(t, tc.myTurn, v.MyTurn, tc.viewer)
		require.Equal(t, tc.score, *v.Score, tc.viewer)
		require.Equal(t, tc.peerName, *v.PeerHandle, tc.viewer)
		require.Nil(t, v.Result)
		require.Equal(t, 13, *v.View.(constellations.View).LastEdge)
		require.Equal(t, []int{}, v.BotEdges)
	}
}

func TestGameViewJSONShape(t *testing.T) {
	g := store.GameRow{ID: "x", Kind: "constellations", Mode: "astralis", Level: 4, Options: game.Options{Level: 4},
		PlayerA: alice, HandleA: "alice", State: raw(t, newState(t, 4, 1)), Status: "active"}
	out, err := json.Marshal(view(t, g, alice))
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(out, &m))
	require.ElementsMatch(t, []string{"id", "kind", "mode", "level", "options", "peerId", "peerHandle", "me",
		"myTurn", "status", "result", "score", "view", "botEdges", "version", "updatedAt"}, keys(m))
	require.Nil(t, m["peerId"])
	require.Equal(t, map[string]any{"level": float64(4)}, m["options"])
	require.Equal(t, map[string]any{"me": float64(0), "them": float64(0)}, m["score"])
	require.Equal(t, []any{}, m["botEdges"])
	v := m["view"].(map[string]any)
	require.ElementsMatch(t, []string{"rows", "cols", "edges", "active", "drawn", "drawnBy", "owner",
		"toMove", "lastEdge"}, keys(v))
	require.Equal(t, float64(6), v["rows"])
	require.Nil(t, v["lastEdge"])
	require.Len(t, v["active"], 25)
}

func keys(m map[string]any) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	return out
}

// a game stored by 0008 (no level in its state, options '{}' and the 0009
// column default of 2) must still load and play
func TestLegacyRowPlays(t *testing.T) {
	legacy := `{"drawn":[true,false,false,true,false,false,false,false,false,false,false,false,
		true,false,false,false,false,false,false,false,false,false,false,false],
		"drawnBy":[1,0,0,2,0,0,0,0,0,0,0,0,1,0,0,0,0,0,0,0,0,0,0,0],
		"owner":[0,0,0,0,0,0,0,0,0],"toMove":1,"lastEdge":12}`
	g := store.GameRow{ID: "33333333-0000-4000-8000-000000000005", Kind: "constellations", Mode: "astralis",
		Level: 2, Options: game.Options{Level: 2}, PlayerA: alice, State: game.State(legacy), Status: "active"}
	v := view(t, g, alice)
	require.True(t, v.MyTurn)
	require.Equal(t, 4, v.View.(constellations.View).Rows)

	next, bot, toMove, err := play(g, alice, edge(13))
	require.NoError(t, err)
	st := decodeState(t, next.State)
	require.Equal(t, int8(1), st.Owner[0], "edge 13 closes box 0 for the person")
	require.Empty(t, bot, "closing a box keeps the turn")
	require.Equal(t, int8(1), toMove)
	require.Equal(t, 2, st.Level)
}

func TestPlayPeer(t *testing.T) {
	g := peerGame(t, newState(t, 2, 1))

	_, _, _, err := play(g, bruno, edge(0))
	require.ErrorIs(t, err, errNotYourTurn)
	_, _, _, err = play(g, alice, edge(99))
	require.ErrorIs(t, err, game.ErrIllegal)
	_, _, _, err = play(g, alice, json.RawMessage(`{"guess":"ORBIT"}`))
	require.ErrorIs(t, err, game.ErrIllegal)

	before := string(g.State)
	next, bot, toMove, err := play(g, alice, edge(0))
	require.NoError(t, err)
	require.Empty(t, bot)
	require.Equal(t, int8(2), toMove)
	require.Equal(t, "active", next.Status)
	require.Equal(t, before, string(g.State), "play must not touch the game it was given")
}

func TestPlayFinishesAndNamesTheWinner(t *testing.T) {
	// everything but the last edge, then whoever is to move draws it
	st := fill(t, newState(t, 2, 1), seq(0, 23)...)
	for _, mode := range []string{"peer", "astralis", "solo"} {
		g := peerGame(t, st)
		g.Mode = mode
		if mode != "peer" {
			g.PlayerB, g.HandleB = nil, nil
			if mode == "astralis" && st.ToMove == 2 {
				continue // astralis never leaves the board on its own turn
			}
		}
		mover := alice
		if mode == "peer" && st.ToMove == 2 {
			mover = bruno
		}
		next, _, toMove, err := play(g, mover, edge(23))
		require.NoError(t, err, mode)
		require.Equal(t, "finished", next.Status, mode)
		require.Zero(t, toMove)
		w := decodeState(t, next.State).Winner()
		switch {
		case mode == "solo":
			require.Nil(t, next.Winner)
			require.Nil(t, view(t, next, alice).Result)
			require.NotNil(t, view(t, next, alice).Score, "a solo board still shows its score")
		case w == 1:
			require.Equal(t, alice, *next.Winner, mode)
			require.Equal(t, "won", *view(t, next, alice).Result, mode)
		case mode == "peer":
			require.Equal(t, bruno, *next.Winner)
			require.Equal(t, "won", *view(t, next, bruno).Result)
			require.Equal(t, "lost", *view(t, next, alice).Result)
		default:
			require.Nil(t, next.Winner, "astralis is never a user, so its win leaves winner NULL")
			require.Equal(t, "lost", *view(t, next, alice).Result)
		}
		require.False(t, view(t, next, alice).MyTurn)
	}
}

// first light can end two boxes each: nobody is named and both read a draw
func TestPlayDraw(t *testing.T) {
	st := fill(t, newState(t, 1, 1), 2, 3, 0, 1, 4, 5, 6, 7, 8, 9, 10)
	g := peerGame(t, st)
	mover := alice
	if st.ToMove == 2 {
		mover = bruno
	}
	next, _, _, err := play(g, mover, edge(11))
	require.NoError(t, err)
	require.Equal(t, "finished", next.Status)
	require.Nil(t, next.Winner)
	for _, who := range []string{alice, bruno} {
		v := view(t, next, who)
		require.Equal(t, "draw", *v.Result)
		require.Equal(t, scoreJSON{Me: 2, Them: 2}, *v.Score)
	}
}

func seq(from, to int) []int {
	out := []int{}
	for i := from; i < to; i++ {
		out = append(out, i)
	}
	return out
}

func TestPlaySoloTakesEitherColour(t *testing.T) {
	g := store.GameRow{ID: "s", Kind: "constellations", Mode: "solo", PlayerA: alice,
		State: raw(t, newState(t, 3, 1)), Status: "active"}
	for range 6 {
		st := decodeState(t, g.State)
		var e int
		for e = range st.Drawn {
			if !st.Drawn[e] {
				break
			}
		}
		var err error
		g, _, _, err = play(g, alice, edge(e))
		require.NoError(t, err)
		v := view(t, g, alice)
		require.True(t, v.MyTurn)
		require.Equal(t, int8(1), v.Me)
	}
}

func TestPlayAstralisReplies(t *testing.T) {
	for level := 1; level <= constellations.MaxLevel; level++ {
		g := store.GameRow{ID: "22222222-0000-4000-8000-000000000004", Kind: "constellations", Mode: "astralis",
			Level: level, Options: game.Options{Level: level}, PlayerA: alice,
			State: raw(t, newState(t, level, 1)), Status: "active"}
		for g.Status == "active" {
			st := decodeState(t, g.State)
			var e int
			for e = range st.Drawn {
				if !st.Drawn[e] {
					if _, err := st.Apply(1, e); err == nil {
						break
					}
				}
			}
			next, bot, toMove, err := play(g, alice, edge(e))
			require.NoError(t, err)
			after := decodeState(t, next.State)
			require.Equal(t, int8(1), after.DrawnBy[e])
			for _, b := range bot {
				require.False(t, st.Drawn[b])
				require.Equal(t, int8(2), after.DrawnBy[b], "astralis draws as player 2")
			}
			if next.Status == "active" {
				require.Equal(t, int8(1), toMove, "the person always gets the board back")
			}
			g = next
		}
		require.True(t, decodeState(t, g.State).Over(), "level %d", level)
	}

	// the same game id and the same moves give the same replies
	again := store.GameRow{ID: "22222222-0000-4000-8000-000000000004", Kind: "constellations", Mode: "astralis",
		PlayerA: alice, State: raw(t, newState(t, 2, 1)), Status: "active"}
	_, b1, _, _ := play(again, alice, edge(0))
	_, b2, _, _ := play(again, alice, edge(0))
	require.Equal(t, b1, b2)
}

func TestUnlockedLevel(t *testing.T) {
	cases := []struct{ botOrSolo, anyMode, want int }{
		{0, 0, 1}, // nobody has finished anything
		{1, 1, 2}, // first light done
		{0, 2, 2}, // only ever played people on the old board
		{2, 2, 3}, // an old astralis game opens lyra
		{3, 4, 4}, // a peer game on orion keeps orion open
		{5, 5, 5}, // capped
		{4, 0, 5}, // inconsistent input still lands in range
		{0, 9, 5}, // as does a level from the future
		{-1, -1, 1},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, unlockedLevel(tc.botOrSolo, tc.anyMode), "%+v", tc)
	}
	require.Equal(t, 2, peerLevelCap(1, 5))
	require.Equal(t, 3, peerLevelCap(3, 4))
	require.Equal(t, 2, peerLevelCap(1, 1))
}

// a fake puzzle engine with two seats that both move at once, for the view
// rules every puzzle and duel shares: no score in solo, solved or unsolved,
// and myTurn following SeatDone
type fakePuzzle struct{}

type fakeState struct {
	Done   [2]bool `json:"done"`
	Solved bool    `json:"solved"`
}

func (fakePuzzle) Kind() string { return "fake" }
func (fakePuzzle) Seats() int   { return 2 }
func (fakePuzzle) New(uint64, game.Options, int) (game.State, error) {
	return json.Marshal(fakeState{})
}
func (fakePuzzle) Apply(s game.State, seat int8, m json.RawMessage) (game.State, error) {
	var st fakeState
	_ = json.Unmarshal(s, &st)
	if st.Done[seat-1] {
		return nil, game.ErrIllegal
	}
	st.Done[seat-1] = true
	st.Solved = string(m) == `"right"`
	return json.Marshal(st)
}
func (fakePuzzle) ToMove(game.State) int8 { return 0 }
func (fakePuzzle) View(s game.State, _ int8) (any, error) {
	var st fakeState
	return st, json.Unmarshal(s, &st)
}
func (fakePuzzle) Result(s game.State) game.Result {
	var st fakeState
	_ = json.Unmarshal(s, &st)
	r := game.Result{Over: st.Done[0] && st.Done[1], Solved: st.Solved, Score: [2]int{3, 4}}
	if r.Over {
		r.Winner = 1
	}
	return r
}
func (fakePuzzle) SeatDone(s game.State, seat int8) bool {
	var st fakeState
	_ = json.Unmarshal(s, &st)
	return st.Done[seat-1]
}

func withFake(t *testing.T) {
	lookup = func(kind string) (game.Engine, bool) {
		if kind == "fake" {
			return fakePuzzle{}, true
		}
		return registry.Get(kind)
	}
	t.Cleanup(func() { lookup = registry.Get })
}

func TestPuzzleAndDuelViews(t *testing.T) {
	withFake(t)
	s0, _ := fakePuzzle{}.New(0, game.Options{}, 1)

	solo := store.GameRow{ID: "p", Kind: "fake", Mode: "solo", PlayerA: alice, State: s0, Status: "active"}
	v := view(t, solo, alice)
	require.True(t, v.MyTurn)
	require.Nil(t, v.Score, "a puzzle has no opponent to score against")
	solo.State, _ = json.Marshal(fakeState{Done: [2]bool{true, true}, Solved: true})
	settle(fakePuzzle{}, &solo)
	require.Equal(t, "finished", solo.Status)
	require.Equal(t, alice, *solo.Winner, "a solved puzzle names its player, which is how points tell solved from finished")
	require.Equal(t, "solved", *view(t, solo, alice).Result)
	solo.State, _ = json.Marshal(fakeState{Done: [2]bool{true, true}})
	settle(fakePuzzle{}, &solo)
	require.Nil(t, solo.Winner, "finished without solving earns no winner")
	require.Equal(t, "unsolved", *view(t, solo, alice).Result)

	duel := store.GameRow{ID: "d", Kind: "fake", Mode: "peer", PlayerA: alice, PlayerB: strp(bruno),
		HandleB: strp("bruno"), State: s0, Status: "active"}
	next, _, toMove, err := play(duel, bruno, json.RawMessage(`"right"`))
	require.NoError(t, err)
	require.Zero(t, toMove)
	require.True(t, view(t, next, alice).MyTurn, "alice has not played yet")
	require.False(t, view(t, next, bruno).MyTurn, "bruno is done and waits")
	_, _, _, err = play(next, bruno, json.RawMessage(`"again"`))
	require.ErrorIs(t, err, game.ErrIllegal)

	next, _, _, err = play(next, alice, json.RawMessage(`"wrong"`))
	require.NoError(t, err)
	require.Equal(t, "finished", next.Status)
	require.Equal(t, alice, *next.Winner)
	require.Equal(t, "won", *view(t, next, alice).Result)
	require.Equal(t, scoreJSON{Me: 4, Them: 3}, *view(t, next, bruno).Score)
}

func TestUnknownKind(t *testing.T) {
	g := store.GameRow{ID: "u", Kind: "chess", Mode: "solo", PlayerA: alice, State: game.State(`{}`)}
	_, err := gameView(g, alice, nil)
	require.ErrorIs(t, err, errUnknownKind)
	_, _, _, err = play(g, alice, edge(0))
	require.ErrorIs(t, err, errUnknownKind)
}

func TestStartPeer(t *testing.T) {
	g, toMove, err := startPeer(store.InviteRow{Kind: "constellations", Options: game.Options{Level: 3}})
	require.NoError(t, err)
	require.Regexp(t, uuidRe, g.ID)
	require.Equal(t, 3, g.Level)
	require.Equal(t, 3, decodeState(t, g.State).Level)
	require.Contains(t, []int8{1, 2}, toMove)
	_, _, err = startPeer(store.InviteRow{Kind: "constellations", Options: game.Options{Level: 8}})
	require.Error(t, err)
}

// the bot's seed is the 0008 fnv of the game id, so games started before
// levels replay with the bot they began with
func TestSeedOfIsStable(t *testing.T) {
	require.Equal(t, uint64(0xcbf29ce484222325), seedOf(""))
	require.NotEqual(t, seedOf(alice), seedOf(bruno))
}

func TestNewGameID(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		id, err := newGameID()
		require.NoError(t, err)
		require.Regexp(t, uuidRe, id)
		require.Equal(t, byte('4'), id[14])
		require.False(t, seen[id])
		seen[id] = true
	}
}

// the flag decides whether the routes exist at all
func TestGameRoutesBehindFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, on := range []bool{false, true} {
		s := NewServer(&config.Config{GamesEnabled: on}, nil, nil, nil, nil, nil, nil)
		r := s.Router()
		found := map[string]bool{}
		for _, ri := range r.Routes() {
			found[ri.Method+" "+ri.Path] = true
		}
		for _, want := range []string{
			"POST /api/v1/peers/:id/invites", "POST /api/v1/games", "GET /api/v1/games/invites",
			"POST /api/v1/games/invites/:id/accept", "POST /api/v1/games/invites/:id/decline",
			"GET /api/v1/games/active", "GET /api/v1/games/leaderboard", "GET /api/v1/games/:id",
			"POST /api/v1/games/:id/moves", "POST /api/v1/me/leaderboard-opt-in",
			"GET /api/v1/games/catalog", "GET /api/v1/games/progress",
		} {
			require.Equal(t, on, found[want], "%s with games=%v", want, on)
		}
	}
}

// words keeps each seat's progress in its view, so a duel seat that has run
// out of guesses waits while the other still plays
func TestWordsDuelMyTurnFollowsTheView(t *testing.T) {
	e, ok := registry.Get("words")
	require.True(t, ok)
	st, err := e.New(7, game.Options{Topic: "calm", Mode: "classic"}, 2)
	require.NoError(t, err)
	g := store.GameRow{ID: "w", Kind: "words", Mode: "peer", PlayerA: alice, PlayerB: strp(bruno),
		HandleB: strp("bruno"), State: st, Status: "active"}
	length := int(outcomeLength(t, view(t, g, bruno).View))
	guess, _ := json.Marshal(map[string]string{"guess": strings.Repeat("Q", length)})
	for range 20 {
		if !view(t, g, bruno).MyTurn {
			break
		}
		g, _, _, err = play(g, bruno, guess)
		require.NoError(t, err)
	}
	require.False(t, view(t, g, bruno).MyTurn)
	require.True(t, view(t, g, alice).MyTurn)
	require.Equal(t, "active", g.Status)
}

func outcomeLength(t *testing.T, v any) float64 {
	t.Helper()
	var m map[string]any
	b, err := json.Marshal(v)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(b, &m))
	return m["length"].(float64)
}
