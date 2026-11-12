// Package game holds the rules for constellations: dots and boxes on a 4x4
// star field. it is pure on purpose. no database, no clock, no randomness: the
// server owns persistence and turn order, this package only says whether a move
// is legal and what the board looks like afterwards.
//
// edges are numbered so the frontend can draw them without asking:
//
//	0-11  horizontal, edge = r*3 + c, joining star (r,c) to (r,c+1)
//	12-23 vertical,   edge = 12 + r*4 + c, joining star (r,c) to (r+1,c)
//
// and box b = r*3 + c is the square whose top-left star is (r,c).
package game

import (
	"encoding/json"
	"errors"
)

const (
	N      = 4                 // stars per side
	hEdges = N * (N - 1)       // 12 horizontal
	Edges  = 2 * hEdges        // 24 in all
	Boxes  = (N - 1) * (N - 1) // 9, odd, so a finished game always has a winner
)

// noEdge marks a board nobody has drawn on yet; it serialises as null
const noEdge = -1

// State is a value: Apply returns a new one and never touches its receiver,
// so a rejected move cannot leave a half-applied board behind. arrays rather
// than slices keep it comparable with ==, which the tests lean on
type State struct {
	Drawn    [Edges]bool
	DrawnBy  [Edges]int8 // who drew each edge, 0 for undrawn, so the board can colour lines by player
	Owner    [Boxes]int8 // 0 nobody, 1 player_a, 2 player_b
	ToMove   int8        // 1 or 2; after the last move it is simply whoever would be next
	LastEdge int         // noEdge until the first move
}

var ErrIllegal = errors.New("illegal move")

// New is an empty board with first (1 or 2) to play
func New(first int8) State {
	return State{ToMove: first, LastEdge: noEdge}
}

// wire is the jsonb and api shape; lastEdge is a pointer only here, so State
// itself stays a plain comparable value
type wire struct {
	Drawn    [Edges]bool `json:"drawn"`
	DrawnBy  [Edges]int8 `json:"drawnBy"`
	Owner    [Boxes]int8 `json:"owner"`
	ToMove   int8        `json:"toMove"`
	LastEdge *int        `json:"lastEdge"`
}

func (s State) MarshalJSON() ([]byte, error) {
	w := wire{Drawn: s.Drawn, DrawnBy: s.DrawnBy, Owner: s.Owner, ToMove: s.ToMove}
	if s.LastEdge >= 0 {
		e := s.LastEdge
		w.LastEdge = &e
	}
	return json.Marshal(w)
}

// UnmarshalJSON refuses a stored state the rules could never have produced
// the shape of, rather than letting a corrupt row panic on an index later
func (s *State) UnmarshalJSON(b []byte) error {
	var w wire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	if w.ToMove != 1 && w.ToMove != 2 {
		return errors.New("game state: toMove must be 1 or 2")
	}
	for _, o := range w.Owner {
		if o < 0 || o > 2 {
			return errors.New("game state: owner out of range")
		}
	}
	for e, by := range w.DrawnBy {
		if by < 0 || by > 2 || (by != 0 && !w.Drawn[e]) {
			return errors.New("game state: drawnBy out of range")
		}
	}
	last := noEdge
	if w.LastEdge != nil {
		if *w.LastEdge < 0 || *w.LastEdge >= Edges {
			return errors.New("game state: lastEdge out of range")
		}
		last = *w.LastEdge
	}
	*s = State{Drawn: w.Drawn, DrawnBy: w.DrawnBy, Owner: w.Owner, ToMove: w.ToMove, LastEdge: last}
	return nil
}

// boxesOf returns the boxes an edge borders: one on the rim, two inside
func boxesOf(e int) []int {
	out := make([]int, 0, 2)
	if e < hEdges {
		r, c := e/(N-1), e%(N-1)
		if r > 0 {
			out = append(out, (r-1)*(N-1)+c)
		}
		if r < N-1 {
			out = append(out, r*(N-1)+c)
		}
	} else {
		v := e - hEdges
		r, c := v/N, v%N
		if c > 0 {
			out = append(out, r*(N-1)+c-1)
		}
		if c < N-1 {
			out = append(out, r*(N-1)+c)
		}
	}
	return out
}

// edgesOf is top, bottom, left, right
func edgesOf(b int) [4]int {
	r, c := b/(N-1), b%(N-1)
	return [4]int{r*(N-1) + c, (r+1)*(N-1) + c, hEdges + r*N + c, hEdges + r*N + c + 1}
}

// Apply plays edge e for player p and returns the next state. closing one or
// two boxes claims them and keeps the turn; otherwise it passes
func (s State) Apply(p int8, e int) (State, error) {
	if p != s.ToMove || e < 0 || e >= Edges || s.Drawn[e] || s.Over() {
		return s, ErrIllegal
	}
	s.Drawn[e] = true
	s.DrawnBy[e] = p
	s.LastEdge = e
	claimed := false
	for _, b := range boxesOf(e) {
		if s.Owner[b] == 0 && s.closed(b) {
			s.Owner[b], claimed = p, true
		}
	}
	if !claimed {
		s.ToMove = 3 - p
	}
	return s, nil
}

func (s State) closed(b int) bool {
	for _, e := range edgesOf(b) {
		if !s.Drawn[e] {
			return false
		}
	}
	return true
}

// Over is true once every edge is drawn, which is also when every box is owned
func (s State) Over() bool {
	for _, d := range s.Drawn {
		if !d {
			return false
		}
	}
	return true
}

// Score counts boxes for player 1 and player 2
func (s State) Score() (a, b int) {
	for _, o := range s.Owner {
		switch o {
		case 1:
			a++
		case 2:
			b++
		}
	}
	return a, b
}

// Winner is 1 or 2 once the game is over, 0 before. nine boxes means no ties
func (s State) Winner() int8 {
	if !s.Over() {
		return 0
	}
	if a, b := s.Score(); a > b {
		return 1
	}
	return 2
}
