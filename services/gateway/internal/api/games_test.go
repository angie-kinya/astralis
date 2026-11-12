package api

import (
	"encoding/json"
	"testing"
	"time"

	"astralis/gateway/internal/config"
	"astralis/gateway/internal/game"
	"astralis/gateway/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const (
	alice = "aaaaaaaa-0000-4000-8000-000000000001"
	bruno = "bbbbbbbb-0000-4000-8000-000000000002"
)

func strp(s string) *string { return &s }

func peerGame(st game.State) store.GameRow {
	return store.GameRow{
		ID: "11111111-0000-4000-8000-000000000003", Mode: "peer",
		PlayerA: alice, HandleA: "alice", PlayerB: strp(bruno), HandleB: strp("bruno"),
		State: st, Status: "active", UpdatedAt: time.Unix(0, 0),
	}
}

// fill plays edges in order for whoever is to move
func fill(t *testing.T, st game.State, edges ...int) game.State {
	t.Helper()
	for _, e := range edges {
		var err error
		st, err = st.Apply(st.ToMove, e)
		require.NoError(t, err)
	}
	return st
}

func TestInviteView(t *testing.T) {
	inv := store.InviteRow{ID: "i", FromID: alice, FromHandle: "alice", ToID: bruno, ToHandle: "bruno", Status: "pending"}
	a, b := inviteView(inv, alice), inviteView(inv, bruno)
	require.True(t, a.Mine)
	require.Equal(t, bruno, a.PeerID)
	require.Equal(t, "bruno", a.PeerHandle)
	require.False(t, b.Mine)
	require.Equal(t, alice, b.PeerID)
	require.Equal(t, "alice", b.PeerHandle)

	raw, err := json.Marshal(a)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"gameId":null`)
}

func TestGameViewPerReceiver(t *testing.T) {
	// player 2 closes box 0 with edge 13 and keeps the turn
	st := game.New(1)
	st = fill(t, st, 0, 3, 12, 13)
	require.Equal(t, int8(2), st.Owner[0])
	g := peerGame(st)

	cases := []struct {
		viewer   string
		me       int8
		myTurn   bool
		score    scoreJSON
		peerName string
	}{
		{alice, 1, st.ToMove == 1, scoreJSON{Me: 0, Them: 1}, "bruno"},
		{bruno, 2, st.ToMove == 2, scoreJSON{Me: 1, Them: 0}, "alice"},
	}
	for _, tc := range cases {
		v := gameView(g, tc.viewer, nil)
		require.Equal(t, tc.me, v.Me, tc.viewer)
		require.Equal(t, tc.myTurn, v.MyTurn, tc.viewer)
		require.Equal(t, tc.score, v.Score, tc.viewer)
		require.Equal(t, tc.peerName, *v.PeerHandle, tc.viewer)
		require.Nil(t, v.Result)
		require.Equal(t, 13, *v.LastEdge)
		require.Equal(t, []int{}, v.BotEdges)
	}
}

func TestGameViewJSONShape(t *testing.T) {
	g := store.GameRow{ID: "x", Mode: "astralis", PlayerA: alice, HandleA: "alice",
		State: game.New(1), Status: "active"}
	raw, err := json.Marshal(gameView(g, alice, nil))
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	for _, k := range []string{"id", "mode", "peerId", "peerHandle", "me", "drawn", "drawnBy", "owner", "toMove",
		"myTurn", "score", "status", "result", "lastEdge", "botEdges", "version", "updatedAt"} {
		require.Contains(t, m, k)
	}
	require.Nil(t, m["peerId"])
	require.Nil(t, m["lastEdge"])
	require.Equal(t, []any{}, m["botEdges"])
}

func TestPlayPeer(t *testing.T) {
	g := peerGame(game.New(1))

	_, _, err := play(g, bruno, 0)
	require.ErrorIs(t, err, errNotYourTurn)
	_, _, err = play(g, alice, 99)
	require.ErrorIs(t, err, game.ErrIllegal)

	next, bot, err := play(g, alice, 0)
	require.NoError(t, err)
	require.Empty(t, bot)
	require.Equal(t, int8(2), next.State.ToMove)
	require.Equal(t, "active", next.Status)
	require.False(t, g.State.Drawn[0], "play must not touch the game it was given")
}

func TestPlayFinishesAndNamesTheWinner(t *testing.T) {
	// everything but the last edge, then whoever is to move draws it
	st := fill(t, game.New(1), seq(0, game.Edges-1)...)
	for _, mode := range []string{"peer", "astralis", "solo"} {
		g := peerGame(st)
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
		next, _, err := play(g, mover, game.Edges-1)
		require.NoError(t, err, mode)
		require.Equal(t, "finished", next.Status, mode)
		w := next.State.Winner()
		switch {
		case mode == "solo":
			require.Nil(t, next.Winner)
			require.Nil(t, gameView(next, alice, nil).Result)
		case w == 1:
			require.Equal(t, alice, *next.Winner, mode)
			require.Equal(t, "won", *gameView(next, alice, nil).Result, mode)
		case mode == "peer":
			require.Equal(t, bruno, *next.Winner)
			require.Equal(t, "won", *gameView(next, bruno, nil).Result)
			require.Equal(t, "lost", *gameView(next, alice, nil).Result)
		default:
			require.Nil(t, next.Winner, "astralis is never a user, so its win leaves winner NULL")
			require.Equal(t, "lost", *gameView(next, alice, nil).Result)
		}
		require.False(t, gameView(next, alice, nil).MyTurn)
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
	g := store.GameRow{ID: "s", Mode: "solo", PlayerA: alice, State: game.New(1), Status: "active"}
	for range 4 {
		var err error
		var e int
		for e = range game.Edges {
			if !g.State.Drawn[e] {
				break
			}
		}
		g, _, err = play(g, alice, e)
		require.NoError(t, err)
		require.True(t, gameView(g, alice, nil).MyTurn)
		require.Equal(t, int8(1), gameView(g, alice, nil).Me)
	}
}

func TestPlayAstralisReplies(t *testing.T) {
	g := store.GameRow{ID: "22222222-0000-4000-8000-000000000004", Mode: "astralis",
		PlayerA: alice, State: game.New(1), Status: "active"}
	for g.Status == "active" {
		var e int
		for e = range game.Edges {
			if !g.State.Drawn[e] {
				break
			}
		}
		next, bot, err := play(g, alice, e)
		require.NoError(t, err)
		require.Equal(t, int8(1), next.State.DrawnBy[e])
		for _, b := range bot {
			require.False(t, g.State.Drawn[b])
			require.True(t, next.State.Drawn[b])
			require.Equal(t, int8(2), next.State.DrawnBy[b], "astralis draws as player 2")
		}
		if next.Status == "active" {
			require.Equal(t, int8(1), next.State.ToMove, "the person always gets the board back")
		}
		g = next
	}
	require.True(t, g.State.Over())

	// the same game id and the same moves give the same replies
	again := store.GameRow{ID: g.ID, Mode: "astralis", PlayerA: alice, State: game.New(1), Status: "active"}
	_, b1, _ := play(again, alice, 0)
	_, b2, _ := play(again, alice, 0)
	require.Equal(t, b1, b2)
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
		} {
			require.Equal(t, on, found[want], "%s with games=%v", want, on)
		}
	}
}
