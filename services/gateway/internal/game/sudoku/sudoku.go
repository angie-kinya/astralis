// Package sudoku is a quiet, solo sudoku. puzzles are generated from the seed
// with exactly one solution, graded by the techniques a person would need, and
// checked on the server: the solution stays in the stored state and never
// reaches a view, and the client proves completion by submitting its grid
//
// there is no mistake counter and no clock here. conflicts are the client's to
// show softly; the server only refuses a grid that changes a given
package sudoku

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"

	"astralis/gateway/internal/game"
)

type level struct {
	Level int    `json:"level"`
	Name  string `json:"name"`
	Size  int    `json:"size"`
	// maxTier is the hardest technique a puzzle may need; removal stops short
	// of anything beyond it
	maxTier int
	// wantTier is the technique a puzzle should need, so medium is not just
	// easy with fewer givens. generation retries a few times to meet it
	wantTier int
	// floor is the fewest givens to carve down to. a gentle level keeps more
	// numbers on the board than uniqueness alone would allow
	floor int
}

var levels = []level{
	{1, "mini", 6, tierSingles, tierSingles, 14},
	{2, "easy", 9, tierSingles, tierSingles, 36},
	{3, "medium", 9, tierPairs, tierPairs, 24},
	{4, "hard", 9, tierSearch, tierSearch, 0},
}

func findLevel(l int) (level, bool) {
	for _, lv := range levels {
		if lv.Level == l {
			return lv, true
		}
	}
	return level{}, false
}

// attempts bounds the retries for wantTier. it is what keeps hard generation
// fast and deterministic: after the last attempt the best puzzle so far wins
const attempts = 16

// generate returns a puzzle (0 for empty) and its solution
func generate(seed uint64, lv level) (puzzle, solution []int8) {
	g := geomFor(lv.Size)
	// a fixed stream id keeps sudoku apart from any other engine fed the same seed
	rng := rand.New(rand.NewPCG(seed, 0x7375646f6b75))
	bestTier := -1
	for range attempts {
		sol := make([]int8, g.n*g.n)
		g.count(sol, 1, sol, rng)
		p := g.carve(sol, lv, rng)
		t := g.grade(p)
		if t > bestTier {
			puzzle, solution, bestTier = p, sol, t
		}
		if t >= lv.wantTier {
			break
		}
	}
	return puzzle, solution
}

// carve removes cells in pairs mirrored through the centre, the classic
// symmetric look, keeping a removal only while the puzzle still has one
// solution and needs nothing beyond the level's techniques
func (g *geom) carve(sol []int8, lv level, rng *rand.Rand) []int8 {
	p := append([]int8(nil), sol...)
	last := len(p) - 1
	var order []int
	for i := 0; i <= last-i; i++ {
		order = append(order, i)
	}
	rng.Shuffle(len(order), func(a, b int) { order[a], order[b] = order[b], order[a] })
	givens := len(p)
	for _, i := range order {
		j := last - i
		drop := 2
		if i == j {
			drop = 1
		}
		if givens-drop < lv.floor {
			continue
		}
		p[i], p[j] = 0, 0
		if g.count(p, 2, nil, nil) != 1 || (lv.maxTier < tierSearch && g.grade(p) > lv.maxTier) {
			p[i], p[j] = sol[i], sol[j]
			continue
		}
		givens -= drop
	}
	return p
}

func encode(grid []int8) string {
	b := make([]byte, len(grid))
	for i, v := range grid {
		b[i] = '0' + byte(v)
	}
	return string(b)
}

// parse reads a grid of n*n cells: '0' or '.' for empty, '1'..n for digits
func parse(s string, n int) ([]int8, bool) {
	if len(s) != n*n {
		return nil, false
	}
	out := make([]int8, len(s))
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '0' || c == '.':
		case c >= '1' && int(c-'0') <= n:
			out[i] = int8(c - '0')
		default:
			return nil, false
		}
	}
	return out, true
}

// state is what games.state holds. Entries is the whole board as the player
// last submitted it, givens included, so it reads the same as a move
type state struct {
	Level    int    `json:"level"`
	Size     int    `json:"size"`
	Givens   string `json:"givens"`
	Solution string `json:"solution"`
	Entries  string `json:"entries"`
	Solved   bool   `json:"solved"`
}

// decode refuses any stored state the rules could not have produced
func decode(raw game.State) (*state, error) {
	var s state
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	lv, ok := findLevel(s.Level)
	if !ok || lv.Size != s.Size {
		return nil, errors.New("sudoku state: bad level")
	}
	givens, ok1 := parse(s.Givens, s.Size)
	sol, ok2 := parse(s.Solution, s.Size)
	entries, ok3 := parse(s.Entries, s.Size)
	if !ok1 || !ok2 || !ok3 {
		return nil, errors.New("sudoku state: bad grid")
	}
	for i, v := range givens {
		if sol[i] == 0 || (v != 0 && (v != sol[i] || entries[i] != v)) {
			return nil, errors.New("sudoku state: grids disagree")
		}
	}
	if s.Solved != (s.Entries == s.Solution) {
		return nil, errors.New("sudoku state: solved out of step")
	}
	return &s, nil
}

// Engine is the sudoku game. it has no fields: everything lives in the state
type Engine struct{}

// New returns the sudoku engine for the registry
func New() game.Engine { return Engine{} }

func (Engine) Kind() string { return "sudoku" }
func (Engine) Seats() int   { return 1 }

// New generates the puzzle for a level. error text is shown to people as it
// is, so it is plain and kind
func (Engine) New(seed uint64, o game.Options, players int) (game.State, error) {
	if players != 1 {
		return nil, errors.New("sudoku is a quiet game for one")
	}
	lv, ok := findLevel(o.Level)
	if !ok {
		return nil, errors.New("pick a level to begin: mini, easy, medium or hard")
	}
	p, sol := generate(seed, lv)
	g := encode(p)
	return json.Marshal(state{Level: lv.Level, Size: lv.Size, Givens: g, Solution: encode(sol), Entries: g})
}

type move struct {
	Grid *string `json:"grid"`
}

// Apply takes {"grid":"..."}: the whole board, replacing the player's
// entries. every given must be there unchanged; anything else, conflicts
// included, is the player's to work through. the game is over the moment the
// grid matches the solution
func (Engine) Apply(raw game.State, who int8, mv json.RawMessage) (game.State, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	var m move
	if who != 1 || s.Solved || json.Unmarshal(mv, &m) != nil || m.Grid == nil {
		return nil, game.ErrIllegal
	}
	grid, ok := parse(*m.Grid, s.Size)
	if !ok {
		return nil, game.ErrIllegal
	}
	for i := range grid {
		if c := s.Givens[i]; c != '0' && grid[i] != int8(c-'0') {
			return nil, game.ErrIllegal
		}
	}
	s.Entries = encode(grid)
	s.Solved = s.Entries == s.Solution
	return json.Marshal(s)
}

func (Engine) ToMove(game.State) int8 { return 0 }

// View is SudokuView in the contract. it never carries the solution: while
// the game is active the only digits in it are the givens and the player's.
// Wrong lists the cells the player filled with a digit the solution does not
// have there, the way ordinary sudoku apps mark a mistake in red. it says
// "not this" about cells already filled, never what belongs anywhere
type View struct {
	Size    int    `json:"size"`
	BoxRows int    `json:"boxRows"`
	BoxCols int    `json:"boxCols"`
	Level   int    `json:"level"`
	Givens  string `json:"givens"`
	Entries string `json:"entries"`
	Solved  bool   `json:"solved"`
	Wrong   []int  `json:"wrong"`
}

func (Engine) View(raw game.State, who int8) (any, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	if who != 1 {
		return nil, fmt.Errorf("sudoku: no seat %d", who)
	}
	g := geomFor(s.Size)
	wrong := []int{}
	for i := range s.Entries {
		if c := s.Entries[i]; c != '0' && c != s.Solution[i] {
			wrong = append(wrong, i)
		}
	}
	return View{Size: s.Size, BoxRows: g.boxR, BoxCols: g.boxC, Level: s.Level,
		Givens: s.Givens, Entries: s.Entries, Solved: s.Solved, Wrong: wrong}, nil
}

func (Engine) Result(raw game.State) game.Result {
	s, err := decode(raw)
	if err != nil {
		return game.Result{}
	}
	return game.Result{Over: s.Solved, Solved: s.Solved}
}

// Catalog is {levels:[{level,name,size}]}, served as-is by GET /games/catalog
func (Engine) Catalog() any {
	return struct {
		Levels []level `json:"levels"`
	}{levels}
}

var (
	_ game.Engine     = Engine{}
	_ game.Cataloguer = Engine{}
)
