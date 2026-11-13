package constellations

import (
	"encoding/json"
	"fmt"
	"testing"

	"astralis/gateway/internal/game"

	"github.com/stretchr/testify/require"
)

// newBoardAt draws the given edges without playing them, for setting up positions
func newBoardAt(t testing.TB, level int, toMove int8, edges ...int) State {
	t.Helper()
	s, err := NewState(level, toMove)
	require.NoError(t, err)
	for _, e := range edges {
		s.Drawn[e] = true
	}
	return s
}

// every active box lists four edges, each of those edges names it back, and
// only edges touching the shape are playable
func TestGeometry(t *testing.T) {
	want := map[int][3]int{ // stars per side, boxes in play
		1: {3, 3, 4}, 2: {4, 4, 9}, 3: {5, 5, 16}, 4: {6, 6, 17}, 5: {7, 7, 36},
	}
	for l := 1; l <= MaxLevel; l++ {
		b := boards[l]
		require.Equal(t, want[l], [3]int{b.rows, b.cols, b.activeN}, "level %d", l)
		require.Equal(t, b.rows*(b.cols-1)+(b.rows-1)*b.cols, b.edges)
		seen := map[int]int{}
		for box := range b.boxes {
			if !b.active[box] {
				continue
			}
			for _, e := range b.boxEdges[box] {
				require.Contains(t, b.edgeBoxes[e], box, "level %d edge %d box %d", l, e, box)
				seen[e]++
			}
		}
		require.Len(t, seen, b.playableN)
		for e := range b.edges {
			n := len(b.edgeBoxes[e])
			require.Equal(t, n > 0, b.playable[e])
			require.LessOrEqual(t, n, 2)
			require.Equal(t, n, seen[e])
		}
	}
	// spot checks against the numbering in the contract, unchanged from 0008
	b := boards[2]
	require.Equal(t, [4]int{0, 3, 12, 13}, b.boxEdges[0])
	require.Equal(t, [4]int{4, 7, 17, 18}, b.boxEdges[4])
	require.Equal(t, [4]int{8, 11, 22, 23}, b.boxEdges[8])
	// 7x7: H = 42, box 35 is (5,5)
	require.Equal(t, [4]int{35, 41, 42 + 5*7 + 5, 42 + 5*7 + 6}, boards[5].boxEdges[35])
}

// the orion shape must be one piece, or part of it could never be reached by a chain
func TestOrionMaskIsConnected(t *testing.T) {
	b := boards[4]
	require.GreaterOrEqual(t, b.activeN, 12)
	bc := b.cols - 1
	start := -1
	for box := range b.boxes {
		if b.active[box] {
			start = box
			break
		}
	}
	seen := map[int]bool{start: true}
	queue := []int{start}
	for len(queue) > 0 {
		box := queue[0]
		queue = queue[1:]
		r, c := box/bc, box%bc
		for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
			nr, nc := r+d[0], c+d[1]
			if nr < 0 || nc < 0 || nr >= b.rows-1 || nc >= bc {
				continue
			}
			n := nr*bc + nc
			if b.active[n] && !seen[n] {
				seen[n] = true
				queue = append(queue, n)
			}
		}
	}
	require.Len(t, seen, b.activeN)
}

func TestApply(t *testing.T) {
	cases := []struct {
		name      string
		start     State
		player    int8
		edge      int
		wantErr   bool
		wantOwned map[int]int8
		wantNext  int8
	}{
		{name: "first edge passes the turn", start: newBoardAt(t, 2, 1), player: 1, edge: 5, wantNext: 2},
		{name: "rim edge closes one box and keeps the turn",
			start: newBoardAt(t, 2, 2, 0, 3, 12), player: 2, edge: 13,
			wantOwned: map[int]int8{0: 2}, wantNext: 2},
		{name: "shared edge closes two boxes at once",
			// box 0 is 0,3,12,13 and box 1 is 1,4,13,14; 13 is the wall between
			start: newBoardAt(t, 2, 1, 0, 3, 12, 1, 4, 14), player: 1, edge: 13,
			wantOwned: map[int]int8{0: 1, 1: 1}, wantNext: 1},
		{name: "third side of a box claims nothing",
			start: newBoardAt(t, 2, 1, 0, 3), player: 1, edge: 12, wantNext: 2},
		{name: "wrong player", start: newBoardAt(t, 2, 1), player: 2, edge: 0, wantErr: true},
		{name: "edge already drawn", start: newBoardAt(t, 2, 1, 7), player: 1, edge: 7, wantErr: true},
		{name: "edge below range", start: newBoardAt(t, 2, 1), player: 1, edge: -1, wantErr: true},
		{name: "edge above range", start: newBoardAt(t, 2, 1), player: 1, edge: 24, wantErr: true},
		{name: "player out of range", start: newBoardAt(t, 2, 1), player: 3, edge: 0, wantErr: true},
		// orion: horizontal edge 2 runs along the top of masked box 2 only
		{name: "edge outside the shape", start: newBoardAt(t, 4, 1), player: 1, edge: 2, wantErr: true},
		{name: "edge on the shape", start: newBoardAt(t, 4, 1), player: 1, edge: 1, wantNext: 2},
		{name: "small board closes a corner", start: newBoardAt(t, 1, 1, 0, 2, 6), player: 1, edge: 7,
			wantOwned: map[int]int8{0: 1}, wantNext: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := tc.start.clone()
			got, err := tc.start.Apply(tc.player, tc.edge)
			require.Equal(t, before, tc.start, "receiver must never change")
			if tc.wantErr {
				require.ErrorIs(t, err, game.ErrIllegal)
				return
			}
			require.NoError(t, err)
			require.True(t, got.Drawn[tc.edge])
			require.Equal(t, tc.player, got.DrawnBy[tc.edge])
			require.Equal(t, tc.edge, got.LastEdge)
			require.Equal(t, tc.wantNext, got.ToMove)
			for b := range got.Owner {
				require.Equal(t, tc.wantOwned[b], got.Owner[b], "box %d", b)
			}
		})
	}
}

// playAll draws every playable edge in order for whoever is to move
func playAll(t *testing.T, level int) State {
	t.Helper()
	s := newBoardAt(t, level, 1)
	b := s.board()
	for e := range b.edges {
		if !b.playable[e] {
			continue
		}
		var err error
		s, err = s.Apply(s.ToMove, e)
		require.NoError(t, err)
	}
	return s
}

func TestEveryLevelFinishes(t *testing.T) {
	for l := 1; l <= MaxLevel; l++ {
		s := playAll(t, l)
		require.True(t, s.Over(), "level %d", l)
		a, b := s.Score()
		require.Equal(t, s.board().activeN, a+b, "level %d", l)
		_, err := s.Apply(s.ToMove, 0)
		require.ErrorIs(t, err, game.ErrIllegal)
	}
	require.Zero(t, newBoardAt(t, 2, 1).Winner())
}

// first light has four boxes, so two each is a shared sky: no winner, a draw
func TestDraw(t *testing.T) {
	s := newBoardAt(t, 1, 1)
	// the middle edges split the board into two chains of two; each player
	// opens one chain and the other takes it
	for _, e := range []int{2, 3, 0, 1, 4, 5, 6, 7, 8, 9, 10, 11} {
		var err error
		s, err = s.Apply(s.ToMove, e)
		require.NoError(t, err, "edge %d", e)
	}
	require.True(t, s.Over())
	a, b := s.Score()
	require.Equal(t, 4, a+b)
	if a == b {
		require.Zero(t, s.Winner())
		r := engine{}.Result(mustJSON(t, s))
		require.True(t, r.Over)
		require.True(t, r.Draw)
		require.Zero(t, r.Winner)
		return
	}
	t.Fatalf("expected a draw, got %d-%d", a, b)
}

func mustJSON(t testing.TB, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

func TestJSON(t *testing.T) {
	// a row exactly as 0008 wrote it: no level, fixed 24/24/9 arrays
	legacy := `{"drawn":[true,false,false,true,false,false,false,false,false,false,false,false,
		true,true,false,false,false,false,false,false,false,false,false,false],
		"drawnBy":[1,0,0,2,0,0,0,0,0,0,0,0,1,2,0,0,0,0,0,0,0,0,0,0],
		"owner":[2,0,0,0,0,0,0,0,0],"toMove":2,"lastEdge":13}`
	var old State
	require.NoError(t, json.Unmarshal([]byte(legacy), &old))
	require.Equal(t, 2, old.Level)
	require.Equal(t, int8(2), old.Owner[0])
	require.Equal(t, 13, old.LastEdge)
	next, err := old.Apply(2, 5)
	require.NoError(t, err, "an old game must still play")
	require.Equal(t, int8(1), next.ToMove)

	// the very first rows had no drawnBy at all
	require.NoError(t, json.Unmarshal([]byte(`{"drawn":[false,false,false,false,false,false,false,false,false,false,false,false,
		false,false,false,false,false,false,false,false,false,false,false,false],
		"owner":[0,0,0,0,0,0,0,0,0],"toMove":1,"lastEdge":null}`), &old))
	require.Len(t, old.DrawnBy, 24)

	fresh, err := NewState(1, 2)
	require.NoError(t, err)
	require.JSONEq(t, `{"level":1,"drawn":[false,false,false,false,false,false,false,false,false,false,false,false],
		"drawnBy":[0,0,0,0,0,0,0,0,0,0,0,0],"owner":[0,0,0,0],"toMove":2,"lastEdge":null}`, string(mustJSON(t, fresh)))

	for l := 1; l <= MaxLevel; l++ {
		s := playAll(t, l)
		var back State
		require.NoError(t, json.Unmarshal(mustJSON(t, s), &back))
		require.Equal(t, s, back, "level %d", l)
	}

	for _, bad := range []string{
		`{"drawn":[],"owner":[],"toMove":0,"lastEdge":null}`,
		`{"toMove":1,"lastEdge":24}`,
		`{"toMove":1,"owner":[4]}`,
		`{"toMove":1,"drawnBy":[1]}`,
		`{"level":9,"toMove":1}`,
		// a 3x3 board labelled as level 2
		`{"level":2,"drawn":[false,false,false,false,false,false,false,false,false,false,false,false],
			"owner":[0,0,0,0],"toMove":1,"lastEdge":null}`,
		`not json`,
	} {
		require.Error(t, json.Unmarshal([]byte(bad), &old), bad)
	}

	// orion: an edge drawn outside the shape, or a masked box with an owner
	s := newBoardAt(t, 4, 1)
	s.Drawn[2] = true
	require.Error(t, json.Unmarshal(mustJSON(t, s), &old))
	s = newBoardAt(t, 4, 1)
	s.Owner[2] = 1
	require.Error(t, json.Unmarshal(mustJSON(t, s), &old))
}

func TestBotTakesABoxWhenOffered(t *testing.T) {
	// level 3 is never gentle, so the box is always taken
	s := newBoardAt(t, 3, 2, 0, 4, 20)
	for seed := range uint64(30) {
		e := s.pick(seed)
		require.Equal(t, 21, e, "seed %d", seed)
	}
}

// on the gentle levels the bot now and then leaves a box for the person, but
// only for a safe edge, and never when there is no safe edge to play
func TestBotIsGentleOnEarlyLevels(t *testing.T) {
	s := newBoardAt(t, 2, 2, 0, 3, 12, 7, 10)
	took, left := 0, 0
	for seed := range uint64(200) {
		e := s.pick(seed)
		if e == 13 {
			took++
			continue
		}
		left++
		require.True(t, s.safe(e), "seed %d left the box for unsafe edge %d", seed, e)
	}
	require.Positive(t, took)
	require.Positive(t, left)
	require.Less(t, left, took, "gentle, not careless")

	// first light with every other edge gone unsafe: the box is always taken
	s = newBoardAt(t, 1, 2, 0, 2, 6, 1, 3, 9, 11)
	require.Empty(t, s.safeEdges())
	for seed := range uint64(50) {
		require.Equal(t, 7, s.pick(seed))
	}
}

func TestBotPrefersSafeEdges(t *testing.T) {
	for l := 1; l <= MaxLevel; l++ {
		s := newBoardAt(t, l, 1)
		b := s.board()
		// two sides on the first active box make its other edges traps
		var first int
		for first = range b.boxes {
			if b.active[first] {
				break
			}
		}
		s.Drawn[b.boxEdges[first][0]], s.Drawn[b.boxEdges[first][2]] = true, true
		for seed := range uint64(50) {
			e := s.pick(seed)
			require.False(t, s.Drawn[e])
			require.True(t, b.playable[e])
			require.True(t, s.safe(e), "level %d seed %d chose unsafe edge %d", l, seed, e)
		}
	}
}

// rows 0 and 1 are chains of three; box 8 is a chain of one. with no safe edge
// left, the bot should hand over the single box, not a row
func TestBotSacrificesTheSmallestChain(t *testing.T) {
	s := newBoardAt(t, 2, 1, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 20, 21, 22)
	s.Owner[6], s.Owner[7] = 2, 2
	_, ok := s.completing()
	require.False(t, ok)
	require.Empty(t, s.safeEdges())
	require.Contains(t, []int{11, 23}, s.pick(7))
}

func TestBotIsReproducible(t *testing.T) {
	e := New()
	for l := 1; l <= MaxLevel; l++ {
		play := func() []string {
			var out []string
			s, err := e.New(42, game.Options{Level: l}, 1)
			require.NoError(t, err)
			for e.ToMove(s) != 0 {
				m, err := e.(game.Bot).BotMove(s, 42, game.Options{})
				require.NoError(t, err)
				out = append(out, string(m))
				s, err = e.Apply(s, e.ToMove(s), m)
				require.NoError(t, err)
			}
			_, err = e.(game.Bot).BotMove(s, 42, game.Options{})
			require.ErrorIs(t, err, game.ErrIllegal)
			return out
		}
		require.Equal(t, play(), play(), "level %d", l)
	}
}

func TestEngine(t *testing.T) {
	e := New()
	require.Equal(t, "constellations", e.Kind())
	require.Equal(t, 2, e.Seats())

	for _, bad := range []struct {
		o       game.Options
		players int
	}{{game.Options{}, 1}, {game.Options{Level: 6}, 1}, {game.Options{Level: 2}, 3}} {
		_, err := e.New(1, bad.o, bad.players)
		require.Error(t, err, "%+v", bad)
	}

	even, err := e.New(4, game.Options{Level: 3}, 2)
	require.NoError(t, err)
	odd, err := e.New(5, game.Options{Level: 3}, 2)
	require.NoError(t, err)
	require.Equal(t, int8(1), e.ToMove(even), "the seed's low bit picks the opener")
	require.Equal(t, int8(2), e.ToMove(odd))

	for _, m := range []string{`{}`, `{"edge":"1"}`, `nope`, `{"edge":999}`} {
		_, err := e.Apply(even, 1, json.RawMessage(m))
		require.ErrorIs(t, err, game.ErrIllegal, m)
	}
	_, err = e.Apply(json.RawMessage(`{"toMove":7}`), 1, json.RawMessage(`{"edge":0}`))
	require.Error(t, err)
	require.NotErrorIs(t, err, game.ErrIllegal, "a corrupt row is not the player's fault")

	next, err := e.Apply(even, 1, json.RawMessage(`{"edge":0}`))
	require.NoError(t, err)
	v, err := e.View(next, 2)
	require.NoError(t, err)
	raw := mustJSON(t, v)
	require.JSONEq(t, fmt.Sprintf(`{"rows":5,"cols":5,"edges":40,"active":%s,"drawn":%s,"drawnBy":%s,
		"owner":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"toMove":2,"lastEdge":0}`,
		mustJSON(t, boards[3].active), drawnJSON(40, 0, true), drawnJSON(40, 0, int8(1))), string(raw))

	full := mustJSON(t, playAll(t, 2))
	require.Zero(t, e.ToMove(full))
	r := e.Result(full)
	require.True(t, r.Over)
	require.Equal(t, 9, r.Score[0]+r.Score[1])
	require.NotZero(t, r.Winner, "nine boxes cannot split evenly")

	cat := mustJSON(t, e.(game.Cataloguer).Catalog())
	require.JSONEq(t, `{"levels":[
		{"level":1,"name":"first light","rows":3,"cols":3,"boxes":4},
		{"level":2,"name":"small dipper","rows":4,"cols":4,"boxes":9},
		{"level":3,"name":"lyra","rows":5,"cols":5,"boxes":16},
		{"level":4,"name":"orion","rows":6,"cols":6,"boxes":17},
		{"level":5,"name":"wide sky","rows":7,"cols":7,"boxes":36}]}`, string(cat))
}

// drawnJSON is an n-long json array of zero values with v at index i
func drawnJSON[T bool | int8](n, i int, v T) string {
	out := make([]T, n)
	out[i] = v
	b, _ := json.Marshal(out)
	return string(b)
}

// FuzzRandomGames plays the bytes as a game through the Engine interface: the
// first byte picks the level, the rest either ask the bot for a move or pick
// one of the legal edges, interleaved with illegal attempts that must bounce
// off. when the bytes run out the bot finishes the game, so every input
// reaches a full board, at every level
func FuzzRandomGames(f *testing.F) {
	for l := range byte(MaxLevel) {
		f.Add([]byte{l})
		f.Add([]byte{l, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
		f.Add([]byte{l, 255, 128, 3, 200, 17, 90, 4, 4, 4, 4, 4, 4, 4})
	}
	f.Add([]byte("constellations"))
	eng := New()
	bot := eng.(game.Bot)
	f.Fuzz(func(t *testing.T, data []byte) {
		level := 1
		if len(data) > 0 {
			level = 1 + int(data[0])%MaxLevel
			data = data[1:]
		}
		seed := uint64(len(data))
		raw, err := eng.New(seed, game.Options{Level: level}, 2)
		if err != nil {
			t.Fatal(err)
		}
		b := boards[level]
		for i := 0; eng.ToMove(raw) != 0; i++ {
			s, err := decode(raw)
			if err != nil {
				t.Fatal(err)
			}
			mover := eng.ToMove(raw)
			if mover != s.ToMove {
				t.Fatalf("toMove %d vs %d", mover, s.ToMove)
			}
			before := string(raw)

			// illegal attempts: the other player, a drawn edge, out of range,
			// outside the shape
			illegal := [][2]int{{int(3 - mover), firstUndrawn(s)}, {int(mover), b.edges + i}, {int(mover), -1 - i}}
			if s.LastEdge >= 0 {
				illegal = append(illegal, [2]int{int(mover), s.LastEdge})
			}
			for e := range b.edges {
				if !b.playable[e] {
					illegal = append(illegal, [2]int{int(mover), e})
					break
				}
			}
			for _, a := range illegal {
				if _, err := eng.Apply(raw, int8(a[0]), mustJSON(t, move{Edge: &a[1]})); err != game.ErrIllegal {
					t.Fatalf("level %d: illegal %v gave %v", level, a, err)
				}
			}
			if string(raw) != before {
				t.Fatal("a rejected move changed the state")
			}

			var m json.RawMessage
			if i >= len(data) || data[i]&0x80 != 0 {
				if m, err = bot.BotMove(raw, seed, game.Options{}); err != nil {
					t.Fatal(err)
				}
			} else {
				var legal []int
				for k := range b.edges {
					if b.playable[k] && !s.Drawn[k] {
						legal = append(legal, k)
					}
				}
				m = mustJSON(t, move{Edge: &legal[int(data[i])%len(legal)]})
			}
			next, err := eng.Apply(raw, mover, m)
			if err != nil {
				t.Fatalf("level %d: legal move %s rejected: %v", level, m, err)
			}
			if string(raw) != before {
				t.Fatal("Apply changed its input")
			}
			if _, err := eng.View(next, mover); err != nil {
				t.Fatal(err)
			}
			raw = next
			if i > b.playableN {
				t.Fatalf("level %d did not end within %d moves", level, b.playableN)
			}
		}
		s, err := decode(raw)
		if err != nil {
			t.Fatal(err)
		}
		for box, o := range s.Owner {
			if b.active[box] != (o == 1 || o == 2) {
				t.Fatalf("level %d box %d owner %d active %v", level, box, o, b.active[box])
			}
		}
		for e, by := range s.DrawnBy {
			if b.playable[e] != (by == 1 || by == 2) {
				t.Fatalf("level %d edge %d drawer %d", level, e, by)
			}
		}
		r := eng.Result(raw)
		if !r.Over || r.Score[0]+r.Score[1] != b.activeN || r.Draw != (r.Score[0] == r.Score[1]) || r.Draw == (r.Winner != 0) {
			t.Fatalf("level %d result %+v", level, r)
		}
		if _, err := eng.Apply(raw, s.ToMove, json.RawMessage(`{"edge":0}`)); err != game.ErrIllegal {
			t.Fatal("finished board accepted a move")
		}
	})
}

func firstUndrawn(s State) int {
	b := s.board()
	for e := range b.edges {
		if b.playable[e] && !s.Drawn[e] {
			return e
		}
	}
	return 0
}
