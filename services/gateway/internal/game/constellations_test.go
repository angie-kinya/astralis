package game

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// board draws the given edges without playing them, for setting up positions
func board(toMove int8, edges ...int) State {
	s := New(toMove)
	for _, e := range edges {
		s.Drawn[e] = true
	}
	return s
}

// every box lists four edges, and each of those edges names the box back
func TestGeometry(t *testing.T) {
	seen := map[int]int{}
	for b := range Boxes {
		for _, e := range edgesOf(b) {
			require.Contains(t, boxesOf(e), b, "edge %d of box %d", e, b)
			seen[e]++
		}
	}
	require.Len(t, seen, Edges)
	for e := range Edges {
		n := len(boxesOf(e))
		require.True(t, n == 1 || n == 2, "edge %d borders %d boxes", e, n)
		require.Equal(t, n, seen[e])
	}
	// spot checks against the numbering in the contract
	require.Equal(t, [4]int{0, 3, 12, 13}, edgesOf(0))
	require.Equal(t, [4]int{4, 7, 17, 18}, edgesOf(4))
	require.Equal(t, [4]int{8, 11, 22, 23}, edgesOf(8))
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
		{name: "first edge passes the turn", start: New(1), player: 1, edge: 5, wantNext: 2},
		{name: "rim edge closes one box and keeps the turn",
			start: board(2, 0, 3, 12), player: 2, edge: 13,
			wantOwned: map[int]int8{0: 2}, wantNext: 2},
		{name: "shared edge closes two boxes at once",
			// box 0 is 0,3,12,13 and box 1 is 1,4,13,14; 13 is the wall between
			start: board(1, 0, 3, 12, 1, 4, 14), player: 1, edge: 13,
			wantOwned: map[int]int8{0: 1, 1: 1}, wantNext: 1},
		{name: "third side of a box claims nothing",
			start: board(1, 0, 3), player: 1, edge: 12, wantNext: 2},
		{name: "wrong player", start: New(1), player: 2, edge: 0, wantErr: true},
		{name: "edge already drawn", start: board(1, 7), player: 1, edge: 7, wantErr: true},
		{name: "edge below range", start: New(1), player: 1, edge: -1, wantErr: true},
		{name: "edge above range", start: New(1), player: 1, edge: Edges, wantErr: true},
		{name: "player out of range", start: New(1), player: 3, edge: 0, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := tc.start
			got, err := tc.start.Apply(tc.player, tc.edge)
			require.Equal(t, before, tc.start, "receiver must never change")
			if tc.wantErr {
				require.ErrorIs(t, err, ErrIllegal)
				require.Equal(t, tc.start, got)
				return
			}
			require.NoError(t, err)
			require.True(t, got.Drawn[tc.edge])
			require.Equal(t, tc.player, got.DrawnBy[tc.edge])
			require.Equal(t, tc.edge, got.LastEdge)
			require.Equal(t, tc.wantNext, got.ToMove)
			for b := range Boxes {
				require.Equal(t, tc.wantOwned[b], got.Owner[b], "box %d", b)
			}
		})
	}
}

func TestOverWinnerAndFinishedBoardRejectsMoves(t *testing.T) {
	s := New(1)
	for e := range Edges {
		var err error
		s, err = s.Apply(s.ToMove, e)
		require.NoError(t, err)
	}
	require.True(t, s.Over())
	a, b := s.Score()
	require.Equal(t, Boxes, a+b)
	require.NotZero(t, s.Winner())
	_, err := s.Apply(s.ToMove, 0)
	require.ErrorIs(t, err, ErrIllegal)
	require.Zero(t, New(1).Winner())
}

func TestJSON(t *testing.T) {
	b, err := json.Marshal(New(2))
	require.NoError(t, err)
	require.JSONEq(t, `{"drawn":[false,false,false,false,false,false,false,false,false,false,false,false,
		false,false,false,false,false,false,false,false,false,false,false,false],
		"drawnBy":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],
		"owner":[0,0,0,0,0,0,0,0,0],"toMove":2,"lastEdge":null}`, string(b))

	s, err := board(1, 0, 3, 12).Apply(1, 13)
	require.NoError(t, err)
	b, err = json.Marshal(s)
	require.NoError(t, err)
	var back State
	require.NoError(t, json.Unmarshal(b, &back))
	require.Equal(t, s, back)
	require.Equal(t, 13, back.LastEdge)

	for _, bad := range []string{
		`{"drawn":[],"owner":[],"toMove":0,"lastEdge":null}`,
		`{"toMove":1,"lastEdge":24}`,
		`{"toMove":1,"owner":[4]}`,
		`{"toMove":1,"drawnBy":[1]}`,
		`not json`,
	} {
		require.Error(t, json.Unmarshal([]byte(bad), &back), bad)
	}
}

func TestBotTakesABoxWhenOffered(t *testing.T) {
	s := board(2, 0, 3, 12, 7, 10)
	require.Equal(t, 13, BotMove(s, 1))
}

func TestBotPrefersSafeEdges(t *testing.T) {
	// box 0 already has two sides, so 1, 13 and 3's neighbours are traps
	s := board(1, 0, 12)
	for seed := range uint64(50) {
		e := BotMove(s, seed)
		require.False(t, s.Drawn[e])
		require.True(t, s.safe(e), "seed %d chose unsafe edge %d", seed, e)
	}
}

// rows 0 and 1 are chains of three; box 8 is a chain of one. with no safe edge
// left, the bot should hand over the single box, not a row
func TestBotSacrificesTheSmallestChain(t *testing.T) {
	s := board(1, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 20, 21, 22)
	s.Owner[6], s.Owner[7] = 2, 2
	_, ok := s.completing()
	require.False(t, ok)
	for e := range Edges {
		if !s.Drawn[e] {
			require.False(t, s.safe(e), "edge %d", e)
		}
	}
	require.Contains(t, []int{11, 23}, BotMove(s, 7))
}

func TestBotIsReproducible(t *testing.T) {
	seed := Seed("3f2b8c1e-9a4d-4e2b-8f1a-2c3d4e5f6a7b")
	play := func() []int {
		var out []int
		s := New(1)
		for !s.Over() {
			e := BotMove(s, seed)
			out = append(out, e)
			var err error
			s, err = s.Apply(s.ToMove, e)
			require.NoError(t, err)
		}
		return out
	}
	require.Equal(t, play(), play())
	require.Equal(t, -1, BotMove(func() State {
		s := New(1)
		for e := range Edges {
			s, _ = s.Apply(s.ToMove, e)
		}
		return s
	}(), seed))
}

// FuzzRandomGames plays the bytes as a game: each byte either asks the bot for
// a move or picks one of the legal edges, interleaved with illegal attempts
// that must bounce off. when the bytes run out the bot finishes the game, so
// every input reaches a full board
func FuzzRandomGames(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
	f.Add([]byte{255, 128, 3, 200, 17, 90, 4, 4, 4, 4, 4, 4, 4})
	f.Add([]byte("constellations"))
	f.Fuzz(func(t *testing.T, data []byte) {
		s := New(int8(1 + len(data)%2))
		seed := uint64(len(data))
		for i := 0; !s.Over(); i++ {
			if s.ToMove != 1 && s.ToMove != 2 {
				t.Fatalf("toMove %d", s.ToMove)
			}

			// illegal attempts: the other player, a drawn edge, out of range
			before := s
			if _, err := s.Apply(3-s.ToMove, 0); err == nil {
				t.Fatal("accepted a move from the wrong player")
			}
			if s.LastEdge >= 0 {
				if _, err := s.Apply(s.ToMove, s.LastEdge); err == nil {
					t.Fatal("accepted a drawn edge")
				}
			}
			if _, err := s.Apply(s.ToMove, Edges+i); err == nil {
				t.Fatal("accepted an edge out of range")
			}
			if s != before {
				t.Fatal("a rejected move changed the state")
			}

			var e int
			if i >= len(data) || data[i]&0x80 != 0 {
				e = BotMove(s, seed)
			} else {
				var legal []int
				for k := range Edges {
					if !s.Drawn[k] {
						legal = append(legal, k)
					}
				}
				e = legal[int(data[i])%len(legal)]
			}
			mover := s.ToMove
			next, err := s.Apply(mover, e)
			if err != nil {
				t.Fatalf("legal edge %d rejected: %v", e, err)
			}
			if next.DrawnBy[e] != mover {
				t.Fatalf("edge %d drawn by %d, recorded %d", e, mover, next.DrawnBy[e])
			}
			if s != before {
				t.Fatal("Apply changed its receiver")
			}
			s = next
			if i > Edges {
				t.Fatal("game did not end within 24 moves")
			}
		}
		for b, o := range s.Owner {
			if o != 1 && o != 2 {
				t.Fatalf("box %d unowned at the end", b)
			}
		}
		if s.ToMove != 1 && s.ToMove != 2 {
			t.Fatalf("final toMove %d", s.ToMove)
		}
		for e, by := range s.DrawnBy {
			if by != 1 && by != 2 {
				t.Fatalf("edge %d has no drawer at the end", e)
			}
		}
		if a, b := s.Score(); a+b != Boxes || s.Winner() == 0 {
			t.Fatalf("score %d-%d winner %d", a, b, s.Winner())
		}
		if _, err := s.Apply(s.ToMove, 0); err == nil {
			t.Fatal("finished board accepted a move")
		}
	})
}
