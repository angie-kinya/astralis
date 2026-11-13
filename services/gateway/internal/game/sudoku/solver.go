package sudoku

import (
	"math/bits"
	"math/rand/v2"
)

// geom is the shape of a board: n = boxR*boxC digits, n*n cells. units and
// peers are precomputed once per shape, since the solver touches them in its
// innermost loop
type geom struct {
	n, boxR, boxC int
	units         [][]int // rows, then columns, then boxes
	peers         [][]int // the 20 (9x9) or 12 (6x6) cells sharing a unit
	unitsOf       [][3]int
}

var (
	g6 = newGeom(2, 3)
	g9 = newGeom(3, 3)
)

func geomFor(n int) *geom {
	if n == 6 {
		return g6
	}
	return g9
}

func newGeom(boxR, boxC int) *geom {
	n := boxR * boxC
	g := &geom{n: n, boxR: boxR, boxC: boxC, peers: make([][]int, n*n), unitsOf: make([][3]int, n*n)}
	for r := range n {
		row := make([]int, n)
		for c := range n {
			row[c] = r*n + c
		}
		g.units = append(g.units, row)
	}
	for c := range n {
		col := make([]int, n)
		for r := range n {
			col[r] = r*n + c
		}
		g.units = append(g.units, col)
	}
	// boxes are boxR rows tall and boxC columns wide; a 6x6 has 3 rows of 2 boxes
	for br := 0; br < n; br += boxR {
		for bc := 0; bc < n; bc += boxC {
			var box []int
			for r := br; r < br+boxR; r++ {
				for c := bc; c < bc+boxC; c++ {
					box = append(box, r*n+c)
				}
			}
			g.units = append(g.units, box)
		}
	}
	for u, cells := range g.units {
		for _, i := range cells {
			g.unitsOf[i][u/n] = u
		}
	}
	for i := range n * n {
		seen := map[int]bool{i: true}
		for _, u := range g.unitsOf[i] {
			for _, j := range g.units[u] {
				if !seen[j] {
					seen[j] = true
					g.peers[i] = append(g.peers[i], j)
				}
			}
		}
	}
	return g
}

func (g *geom) full() uint16 { return uint16(1)<<g.n - 1 }

// candidates is, per empty cell, the digits no peer already holds (bit d-1
// for digit d). ok is false if some empty cell has none, or two peers clash
func (g *geom) candidates(grid []int8) (cand []uint16, ok bool) {
	cand = make([]uint16, len(grid))
	for i, v := range grid {
		if v != 0 {
			for _, j := range g.peers[i] {
				if grid[j] == v {
					return nil, false
				}
			}
			continue
		}
		m := g.full()
		for _, j := range g.peers[i] {
			if d := grid[j]; d != 0 {
				m &^= 1 << (d - 1)
			}
		}
		if m == 0 {
			return nil, false
		}
		cand[i] = m
	}
	return cand, true
}

// count returns the number of solutions up to limit, writing the first one
// found into sol when sol is non-nil. it keeps one digit mask per unit and
// branches on the empty cell with the fewest candidates, which keeps a 9x9
// search to a few hundred nodes. rng, when set, shuffles the digit order so
// the same routine also fills a fresh random grid
func (g *geom) count(grid []int8, limit int, sol []int8, rng *rand.Rand) int {
	if _, ok := g.candidates(grid); !ok {
		return 0
	}
	work := append([]int8(nil), grid...)
	used := make([]uint16, len(g.units))
	for i, v := range work {
		if v != 0 {
			for _, u := range g.unitsOf[i] {
				used[u] |= 1 << (v - 1)
			}
		}
	}
	found := 0
	var rec func() bool
	rec = func() bool {
		best, bestN := -1, 99
		var bestM uint16
		for i, v := range work {
			if v != 0 {
				continue
			}
			us := g.unitsOf[i]
			m := g.full() &^ (used[us[0]] | used[us[1]] | used[us[2]])
			c := bits.OnesCount16(m)
			if c == 0 {
				return false
			}
			if c < bestN {
				best, bestN, bestM = i, c, m
				if c == 1 {
					break
				}
			}
		}
		if best < 0 {
			if found == 0 && sol != nil {
				copy(sol, work)
			}
			found++
			return found >= limit
		}
		var buf [9]int8
		ds := buf[:0]
		for m := bestM; m != 0; m &= m - 1 {
			ds = append(ds, int8(bits.TrailingZeros16(m)+1))
		}
		if rng != nil {
			rng.Shuffle(len(ds), func(a, b int) { ds[a], ds[b] = ds[b], ds[a] })
		}
		us := g.unitsOf[best]
		for _, d := range ds {
			bit := uint16(1) << (d - 1)
			work[best] = d
			used[us[0]], used[us[1]], used[us[2]] = used[us[0]]|bit, used[us[1]]|bit, used[us[2]]|bit
			stop := rec()
			used[us[0]], used[us[1]], used[us[2]] = used[us[0]]&^bit, used[us[1]]&^bit, used[us[2]]&^bit
			if stop {
				work[best] = 0
				return true
			}
		}
		work[best] = 0
		return false
	}
	rec()
	return found
}

// tier is how hard a puzzle is to solve by reasoning alone
const (
	tierSingles = 1 // naked and hidden singles
	tierPairs   = 2 // plus pointing, box/line claiming and naked pairs
	tierSearch  = 3 // needs more than that: chains, guessing, or patience
)

// grade solves like a person would, using the gentlest technique that makes
// progress at each step, and returns the hardest tier it needed. a puzzle the
// techniques cannot finish is tierSearch. this is the grader the generator
// leans on to keep easy puzzles easy
func (g *geom) grade(grid []int8) int {
	work := append([]int8(nil), grid...)
	cand, ok := g.candidates(work)
	if !ok {
		return tierSearch
	}
	place := func(i int, d int8) {
		work[i], cand[i] = d, 0
		for _, j := range g.peers[i] {
			cand[j] &^= 1 << (d - 1)
		}
	}
	tier := tierSingles
	for {
		empty := 0
		for _, v := range work {
			if v == 0 {
				empty++
			}
		}
		if empty == 0 {
			return tier
		}
		if g.singles(work, cand, place) {
			continue
		}
		if g.pairs(cand) {
			tier = tierPairs
			continue
		}
		return tierSearch
	}
}

// singles places every naked single, else the first hidden single it finds
func (g *geom) singles(work []int8, cand []uint16, place func(int, int8)) bool {
	progress := false
	for i, m := range cand {
		if work[i] == 0 && bits.OnesCount16(m) == 1 {
			place(i, int8(bits.TrailingZeros16(m)+1))
			progress = true
		}
	}
	if progress {
		return true
	}
	for _, u := range g.units {
		for d := range g.n {
			bit := uint16(1) << d
			at, hits := -1, 0
			for _, i := range u {
				if cand[i]&bit != 0 {
					at, hits = i, hits+1
				}
			}
			if hits == 1 {
				place(at, int8(d+1))
				return true
			}
		}
	}
	return false
}

// pairs removes candidates by pointing (a digit confined to one line within a
// box leaves the rest of that line), claiming (confined to one box within a
// line leaves the rest of that box) and naked pairs. true if anything went
func (g *geom) pairs(cand []uint16) bool {
	changed := false
	for u, cells := range g.units {
		for d := range g.n {
			bit := uint16(1) << d
			var where []int
			for _, i := range cells {
				if cand[i]&bit != 0 {
					where = append(where, i)
				}
			}
			if len(where) < 2 {
				continue
			}
			// whichever other unit kind all of where shares, clear the digit there
			for kind := range 3 {
				if kind == u/g.n {
					continue
				}
				other := g.unitsOf[where[0]][kind]
				shared := true
				for _, i := range where[1:] {
					if g.unitsOf[i][kind] != other {
						shared = false
						break
					}
				}
				if !shared {
					continue
				}
				for _, j := range g.units[other] {
					if g.unitsOf[j][u/g.n] != u && cand[j]&bit != 0 {
						cand[j] &^= bit
						changed = true
					}
				}
			}
		}
		for a, i := range cells {
			if bits.OnesCount16(cand[i]) != 2 {
				continue
			}
			for _, j := range cells[a+1:] {
				if cand[j] != cand[i] {
					continue
				}
				for _, k := range cells {
					if k != i && k != j && cand[k]&cand[i] != 0 {
						cand[k] &^= cand[i]
						changed = true
					}
				}
			}
		}
	}
	return changed
}
