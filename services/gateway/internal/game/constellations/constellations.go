// Package constellations is dots and boxes on a field of stars, in five
// sizes. it is pure on purpose. no database, no clock, no randomness beyond a
// seed: the server owns persistence and turn order, this package only says
// whether a move is legal and what the board looks like afterwards.
//
// a board is rows x cols stars. edges are numbered so the frontend can draw
// them without asking, horizontal first:
//
//	e = r*(cols-1) + c        joins (r,c)-(r,c+1), r < rows, c < cols-1
//	e = H + r*cols + c        joins (r,c)-(r+1,c), H = rows*(cols-1)
//
// and box b = r*(cols-1) + c is the square whose top-left star is (r,c). a
// level may mask boxes out (orion); an edge is playable only if it borders at
// least one active box, and only active boxes can be claimed
package constellations

import (
	"encoding/json"
	"errors"
	"fmt"

	"astralis/gateway/internal/game"
)

// noEdge marks a board nobody has drawn on yet; it serialises as null
const noEdge = -1

// legacyLevel is the board every game stored before levels existed (0008):
// 4x4 stars, its state json has no level field
const legacyLevel = 2

// MaxLevel is the widest sky
const MaxLevel = 5

type levelDef struct {
	name       string
	rows, cols int
	// mask is one string per box row, '#' active and '.' not; nil means
	// every box is in play
	mask []string
}

// the orion mask is an hourglass on 5x5 boxes: shoulders, the belt through the
// middle, feet. 17 boxes, edge-connected, so every part of the shape can be
// reached by a chain
var levelDefs = [MaxLevel + 1]levelDef{
	1: {name: "first light", rows: 3, cols: 3},
	2: {name: "small dipper", rows: 4, cols: 4},
	3: {name: "lyra", rows: 5, cols: 5},
	4: {name: "orion", rows: 6, cols: 6, mask: []string{
		"##.##",
		".###.",
		".###.",
		".###.",
		"##.##",
	}},
	5: {name: "wide sky", rows: 7, cols: 7},
}

// board is a level's geometry, worked out once at start-up
type board struct {
	rows, cols int
	h          int // horizontal edge count; vertical edges start here
	edges      int
	boxes      int
	active     []bool   // per box
	activeN    int      // boxes in play
	playable   []bool   // per edge: borders an active box
	playableN  int      // edges in play
	edgeBoxes  [][]int  // per edge: the active boxes it borders (one or two)
	boxEdges   [][4]int // per box: top, bottom, left, right
}

var boards [MaxLevel + 1]*board

func init() {
	for l := 1; l <= MaxLevel; l++ {
		boards[l] = newBoard(levelDefs[l])
	}
}

func newBoard(d levelDef) *board {
	bc := d.cols - 1
	b := &board{rows: d.rows, cols: d.cols, h: d.rows * bc}
	b.edges = b.h + (d.rows-1)*d.cols
	b.boxes = (d.rows - 1) * bc
	b.active = make([]bool, b.boxes)
	b.boxEdges = make([][4]int, b.boxes)
	b.playable = make([]bool, b.edges)
	b.edgeBoxes = make([][]int, b.edges)
	for box := range b.boxes {
		r, c := box/bc, box%bc
		b.active[box] = d.mask == nil || d.mask[r][c] == '#'
		b.boxEdges[box] = [4]int{r*bc + c, (r+1)*bc + c, b.h + r*d.cols + c, b.h + r*d.cols + c + 1}
		if !b.active[box] {
			continue
		}
		b.activeN++
		for _, e := range b.boxEdges[box] {
			b.edgeBoxes[e] = append(b.edgeBoxes[e], box)
			if !b.playable[e] {
				b.playable[e] = true
				b.playableN++
			}
		}
	}
	return b
}

// State is a value: Apply returns a new one with fresh slices and never
// touches its receiver, so a rejected move cannot leave a half-applied board
type State struct {
	Level    int
	Drawn    []bool
	DrawnBy  []int8 // who drew each edge, 0 for undrawn, so the board can colour lines by player
	Owner    []int8 // per box: 0 nobody, 1 seat one, 2 seat two
	ToMove   int8   // 1 or 2; after the last move it is simply whoever would be next
	LastEdge int    // noEdge until the first move
}

// NewState is an empty board at level with first (1 or 2) to play
func NewState(level int, first int8) (State, error) {
	if level < 1 || level > MaxLevel {
		// worded for people: the api shows an engine's New error as it is
		return State{}, fmt.Errorf("There is no level %d in constellations.", level)
	}
	if first != 1 && first != 2 {
		return State{}, fmt.Errorf("constellations: first %d", first)
	}
	b := boards[level]
	return State{
		Level: level, Drawn: make([]bool, b.edges), DrawnBy: make([]int8, b.edges),
		Owner: make([]int8, b.boxes), ToMove: first, LastEdge: noEdge,
	}, nil
}

func (s State) board() *board { return boards[s.Level] }

func (s State) clone() State {
	s.Drawn = append([]bool(nil), s.Drawn...)
	s.DrawnBy = append([]int8(nil), s.DrawnBy...)
	s.Owner = append([]int8(nil), s.Owner...)
	return s
}

// wire is the jsonb shape. level is omitted from nothing: a row written before
// levels existed simply lacks it, and reads back as the 4x4 board it was
type wire struct {
	Level    int    `json:"level"`
	Drawn    []bool `json:"drawn"`
	DrawnBy  []int8 `json:"drawnBy"`
	Owner    []int8 `json:"owner"`
	ToMove   int8   `json:"toMove"`
	LastEdge *int   `json:"lastEdge"`
}

func (s State) MarshalJSON() ([]byte, error) {
	w := wire{Level: s.Level, Drawn: s.Drawn, DrawnBy: s.DrawnBy, Owner: s.Owner, ToMove: s.ToMove}
	if s.LastEdge >= 0 {
		e := s.LastEdge
		w.LastEdge = &e
	}
	return json.Marshal(w)
}

// UnmarshalJSON refuses a stored state the rules could never have produced
// the shape of, rather than letting a corrupt row panic on an index later
func (s *State) UnmarshalJSON(raw []byte) error {
	var w wire
	if err := json.Unmarshal(raw, &w); err != nil {
		return err
	}
	if w.Level == 0 {
		w.Level = legacyLevel
	}
	if w.Level < 1 || w.Level > MaxLevel {
		return errors.New("game state: level out of range")
	}
	b := boards[w.Level]
	if w.ToMove != 1 && w.ToMove != 2 {
		return errors.New("game state: toMove must be 1 or 2")
	}
	if len(w.Drawn) != b.edges || len(w.Owner) != b.boxes {
		return errors.New("game state: board size does not match its level")
	}
	// the very first 0008 rows had no drawnBy; their lines simply lose colour
	if w.DrawnBy == nil {
		w.DrawnBy = make([]int8, b.edges)
	}
	if len(w.DrawnBy) != b.edges {
		return errors.New("game state: drawnBy size does not match its level")
	}
	for i, o := range w.Owner {
		if o < 0 || o > 2 || (o != 0 && !b.active[i]) {
			return errors.New("game state: owner out of range")
		}
	}
	for e, by := range w.DrawnBy {
		if by < 0 || by > 2 || (by != 0 && !w.Drawn[e]) {
			return errors.New("game state: drawnBy out of range")
		}
		if w.Drawn[e] && !b.playable[e] {
			return errors.New("game state: edge outside the shape")
		}
	}
	last := noEdge
	if w.LastEdge != nil {
		if *w.LastEdge < 0 || *w.LastEdge >= b.edges {
			return errors.New("game state: lastEdge out of range")
		}
		last = *w.LastEdge
	}
	*s = State{Level: w.Level, Drawn: w.Drawn, DrawnBy: w.DrawnBy, Owner: w.Owner, ToMove: w.ToMove, LastEdge: last}
	return nil
}

// Apply plays edge e for player p and returns the next state. closing one or
// two boxes claims them and keeps the turn; otherwise it passes
func (s State) Apply(p int8, e int) (State, error) {
	b := s.board()
	if p != s.ToMove || e < 0 || e >= b.edges || !b.playable[e] || s.Drawn[e] || s.Over() {
		return s, game.ErrIllegal
	}
	s = s.clone()
	s.Drawn[e] = true
	s.DrawnBy[e] = p
	s.LastEdge = e
	claimed := false
	for _, box := range b.edgeBoxes[e] {
		if s.Owner[box] == 0 && s.closed(box) {
			s.Owner[box], claimed = p, true
		}
	}
	if !claimed {
		s.ToMove = 3 - p
	}
	return s, nil
}

func (s State) closed(box int) bool { return s.sides(box) == 4 }

func (s State) sides(box int) int {
	n := 0
	for _, e := range s.board().boxEdges[box] {
		if s.Drawn[e] {
			n++
		}
	}
	return n
}

func (s State) drawnCount() int {
	n := 0
	for _, d := range s.Drawn {
		if d {
			n++
		}
	}
	return n
}

// Over is true once every playable edge is drawn, which is also when every
// active box is owned
func (s State) Over() bool { return s.drawnCount() == s.board().playableN }

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

// Winner is 1 or 2 once the game is over, 0 before and on a shared sky
func (s State) Winner() int8 {
	if !s.Over() {
		return 0
	}
	switch a, b := s.Score(); {
	case a > b:
		return 1
	case b > a:
		return 2
	}
	return 0
}
