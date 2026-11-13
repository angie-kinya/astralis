package sudoku

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"astralis/gateway/internal/game"
)

func grid(t testing.TB, s string, n int) []int8 {
	t.Helper()
	g, ok := parse(s, n)
	if !ok {
		t.Fatalf("bad grid %q", s)
	}
	return g
}

// known puzzles with their published solutions
var known = []struct {
	name, puzzle, solution string
	size, tier             int
}{
	{
		"wikipedia", // the example on the sudoku article, solvable with singles
		"53..7....6..195....98....6.8...6...34..8.3..17...2...6.6....28....419..5....8..79",
		"534678912672195348198342567859761423426853791713924856961537284287419635345286179",
		9, tierSingles,
	},
	{
		"inkala", // arto inkala's 2012 "hardest", which no simple technique cracks
		"8..........36......7..9.2...5...7.......457.....1...3...1....68..85...1..9....4..",
		"812753649943682175675491283154237896369845721287169534521974368438526917796318452",
		9, tierSearch,
	},
	{
		"mini", // a 6x6 latin square with 2x3 boxes, a diagonal and two corners lifted
		".2345.4.612323.564564.31312645.4531.",
		"123456456123231564564231312645645312",
		6, tierSingles,
	},
}

func TestSolverOnKnownPuzzles(t *testing.T) {
	for _, k := range known {
		t.Run(k.name, func(t *testing.T) {
			g := geomFor(k.size)
			p := grid(t, k.puzzle, k.size)
			sol := make([]int8, len(p))
			if n := g.count(p, 2, sol, nil); n != 1 {
				t.Fatalf("%d solutions, want 1", n)
			}
			if encode(sol) != k.solution {
				t.Fatalf("solved to %s", encode(sol))
			}
			if tier := g.grade(p); tier != k.tier {
				t.Fatalf("graded %d, want %d", tier, k.tier)
			}
		})
	}
}

func TestSolverCountsStopAtLimit(t *testing.T) {
	if n := g9.count(make([]int8, 81), 2, nil, nil); n != 2 {
		t.Fatalf("empty board: %d, want the limit 2", n)
	}
	clash := grid(t, "11"+strings.Repeat("0", 79), 9)
	if n := g9.count(clash, 2, nil, nil); n != 0 {
		t.Fatalf("clashing givens: %d solutions", n)
	}
}

func TestGeneratedPuzzles(t *testing.T) {
	for _, lv := range levels {
		t.Run(lv.Name, func(t *testing.T) {
			g := geomFor(lv.Size)
			met := 0
			const seeds = 30
			for seed := range uint64(seeds) {
				p, sol := generate(seed, lv)
				out := make([]int8, len(p))
				if n := g.count(p, 2, out, nil); n != 1 || encode(out) != encode(sol) {
					t.Fatalf("seed %d: %d solutions", seed, n)
				}
				givens := 0
				for i, v := range p {
					if v != 0 {
						givens++
						if v != sol[i] {
							t.Fatalf("seed %d: given disagrees with solution", seed)
						}
					}
					if (v == 0) != (p[len(p)-1-i] == 0) {
						t.Fatalf("seed %d: not symmetric at %d", seed, i)
					}
				}
				if givens < lv.floor {
					t.Fatalf("seed %d: %d givens, floor %d", seed, givens, lv.floor)
				}
				tier := g.grade(p)
				if tier > lv.maxTier {
					t.Fatalf("seed %d: needs tier %d, level allows %d", seed, tier, lv.maxTier)
				}
				if tier >= lv.wantTier {
					met++
				}
			}
			// retries are bounded, so a rare puzzle may fall a tier short
			if met < seeds*8/10 {
				t.Fatalf("only %d of %d puzzles reached tier %d", met, seeds, lv.wantTier)
			}
		})
	}
}

func mustNew(t testing.TB, seed uint64, l int) game.State {
	t.Helper()
	s, err := Engine{}.New(seed, game.Options{Level: l}, 1)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func secret(t testing.TB, s game.State) *state {
	t.Helper()
	st, err := decode(s)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestNewValidates(t *testing.T) {
	for _, l := range []int{0, 5, -1} {
		if _, err := (Engine{}).New(1, game.Options{Level: l}, 1); err == nil {
			t.Errorf("level %d accepted", l)
		}
	}
	if _, err := (Engine{}).New(1, game.Options{Level: 2}, 2); err == nil {
		t.Error("two players accepted")
	}
	a, b := mustNew(t, 99, 4), mustNew(t, 99, 4)
	if string(a) != string(b) {
		t.Fatal("same seed, different puzzles")
	}
	if string(mustNew(t, 100, 4)) == string(a) {
		t.Fatal("different seeds, same puzzle")
	}
}

func TestApply(t *testing.T) {
	s := mustNew(t, 5, 1)
	st := secret(t, s)
	solution := st.Solution
	firstGiven := strings.IndexFunc(st.Givens, func(r rune) bool { return r != '0' })
	firstEmpty := strings.IndexByte(st.Givens, '0')
	with := func(base string, i int, c byte) string {
		b := []byte(base)
		b[i] = c
		return string(b)
	}
	other := byte('1')
	if solution[firstGiven] == '1' {
		other = '2'
	}

	cases := []struct {
		name string
		grid string
		ok   bool
	}{
		{"short", st.Givens[1:], false},
		{"digit too big for 6x6", with(st.Givens, firstEmpty, '7'), false},
		{"letter", with(st.Givens, firstEmpty, 'x'), false},
		{"given changed", with(st.Givens, firstGiven, other), false},
		{"given cleared", with(st.Givens, firstGiven, '.'), false},
		{"dots for empty", strings.ReplaceAll(st.Givens, "0", "."), true},
		{"a wrong digit is fine", with(st.Givens, firstEmpty, other), true},
	}
	for _, c := range cases {
		mv, _ := json.Marshal(map[string]string{"grid": c.grid})
		_, err := Engine{}.Apply(s, 1, mv)
		if c.ok != (err == nil) || (err != nil && !errors.Is(err, game.ErrIllegal)) {
			t.Errorf("%s: err %v", c.name, err)
		}
	}
	for _, mv := range []string{`{}`, `[]`, `{"grid":5}`} {
		if _, err := (Engine{}).Apply(s, 1, json.RawMessage(mv)); !errors.Is(err, game.ErrIllegal) {
			t.Errorf("%s: err %v", mv, err)
		}
	}
	if _, err := (Engine{}).Apply(s, 2, json.RawMessage(`{"grid":"`+solution+`"}`)); !errors.Is(err, game.ErrIllegal) {
		t.Error("seat 2 played")
	}

	s, err := Engine{}.Apply(s, 1, json.RawMessage(`{"grid":"`+solution+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	if r := (Engine{}).Result(s); !r.Over || !r.Solved {
		t.Fatalf("result %+v", r)
	}
	if _, err := (Engine{}).Apply(s, 1, json.RawMessage(`{"grid":"`+solution+`"}`)); !errors.Is(err, game.ErrIllegal) {
		t.Error("played after solving")
	}
}

func TestViewHidesSolution(t *testing.T) {
	for _, lv := range levels {
		s := mustNew(t, 8, lv.Level)
		v, err := Engine{}.View(s, 1)
		if err != nil {
			t.Fatal(err)
		}
		vw := v.(View)
		if vw.Size != lv.Size || vw.BoxRows*vw.BoxCols != lv.Size || vw.Solved {
			t.Fatalf("%s: view %+v", lv.Name, vw)
		}
		b, _ := json.Marshal(v)
		if strings.Contains(string(b), secret(t, s).Solution) || strings.Contains(string(b), "solution") {
			t.Fatalf("%s: view leaks the solution: %s", lv.Name, b)
		}
	}
	if v, _ := (Engine{}).View(mustNew(t, 1, 1), 1); v.(View).BoxRows != 2 || v.(View).BoxCols != 3 {
		t.Fatal("mini boxes should be 2 rows by 3 columns")
	}
}

func TestDecodeRefusesCorruptState(t *testing.T) {
	good := *secret(t, mustNew(t, 3, 2))
	cases := map[string]func(s *state){
		"level":          func(s *state) { s.Level = 9 },
		"size":           func(s *state) { s.Size = 6 },
		"short solution": func(s *state) { s.Solution = s.Solution[1:] },
		"solved flag":    func(s *state) { s.Solved = true },
		"given lost": func(s *state) {
			i := strings.IndexFunc(s.Givens, func(r rune) bool { return r != '0' })
			s.Entries = s.Entries[:i] + "0" + s.Entries[i+1:]
		},
	}
	for name, f := range cases {
		c := good
		f(&c)
		raw, _ := json.Marshal(c)
		if _, err := (Engine{}).View(raw, 1); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestCatalog(t *testing.T) {
	b, _ := json.Marshal(Engine{}.Catalog())
	want := `{"levels":[{"level":1,"name":"mini","size":6},{"level":2,"name":"easy","size":9},` +
		`{"level":3,"name":"medium","size":9},{"level":4,"name":"hard","size":9}]}`
	if string(b) != want {
		t.Fatalf("catalog %s", b)
	}
}

// FuzzMoves submits grids built from arbitrary bytes, mixed with the real
// solution's digits, and checks nothing panics and the solution never shows
// in a view until the puzzle is solved
func FuzzMoves(f *testing.F) {
	f.Add(uint64(1), uint8(1), []byte("123456789"), uint8(3))
	f.Add(uint64(9), uint8(4), []byte{0, '.', 'x', 200}, uint8(80))
	f.Add(uint64(3), uint8(2), []byte{}, uint8(255))
	f.Fuzz(func(t *testing.T, seed uint64, l uint8, noise []byte, keep uint8) {
		lv := levels[int(l)%len(levels)]
		s := mustNew(t, seed%64, lv.Level)
		sol := secret(t, s).Solution
		for round := range 4 {
			g := []byte(sol)
			for i := range g {
				// keep a share of the solution's digits, fill the rest with noise
				if (i*7+round*13)%256 >= int(keep) && len(noise) > 0 {
					g[i] = noise[(i+round)%len(noise)]
				}
			}
			mv, _ := json.Marshal(map[string]string{"grid": string(g)})
			if next, err := (Engine{}).Apply(s, 1, mv); err == nil {
				s = next
			} else if !errors.Is(err, game.ErrIllegal) {
				t.Fatal(err)
			}
			v, err := Engine{}.View(s, 1)
			if err != nil {
				t.Fatal(err)
			}
			b, _ := json.Marshal(v)
			solved := Engine{}.Result(s).Solved
			if !solved && strings.Contains(string(b), sol) {
				t.Fatalf("solution in an unsolved view: %s", b)
			}
			if solved != v.(View).Solved {
				t.Fatal("view and result disagree")
			}
		}
	})
}

func BenchmarkGenerate(b *testing.B) {
	for _, lv := range levels {
		b.Run(lv.Name, func(b *testing.B) {
			for i := range b.N {
				generate(uint64(i), lv)
			}
		})
	}
}
