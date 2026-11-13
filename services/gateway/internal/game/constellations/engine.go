package constellations

import (
	"encoding/json"
	"fmt"

	"astralis/gateway/internal/game"
)

// Kind is the games.kind value for constellations
const Kind = "constellations"

type engine struct{}

// New is the constellations engine: game.Engine, game.Bot and game.Cataloguer
func New() game.Engine { return engine{} }

var (
	_ game.Bot        = engine{}
	_ game.Cataloguer = engine{}
)

func (engine) Kind() string { return Kind }
func (engine) Seats() int   { return 2 }

// New deals an empty board. the low bit of the seed picks who opens, so a
// caller that wants seat 1 first (solo, one person with both colours) clears it
func (engine) New(seed uint64, o game.Options, players int) (game.State, error) {
	if players != 1 && players != 2 {
		return nil, fmt.Errorf("constellations: %d players", players)
	}
	st, err := NewState(o.Level, int8(1+seed&1))
	if err != nil {
		return nil, err
	}
	return json.Marshal(st)
}

func decode(s game.State) (State, error) {
	var st State
	if err := json.Unmarshal(s, &st); err != nil {
		return State{}, fmt.Errorf("constellations: %w", err)
	}
	return st, nil
}

// move is {"edge": n}; a pointer so edge 0 is not mistaken for a missing field
type move struct {
	Edge *int `json:"edge"`
}

func (engine) Apply(s game.State, seat int8, m json.RawMessage) (game.State, error) {
	st, err := decode(s)
	if err != nil {
		return nil, err
	}
	var mv move
	if json.Unmarshal(m, &mv) != nil || mv.Edge == nil {
		return nil, game.ErrIllegal
	}
	next, err := st.Apply(seat, *mv.Edge)
	if err != nil {
		return nil, err
	}
	return json.Marshal(next)
}

// ToMove is 0 once the board is full; a corrupt state reads as nobody's turn,
// and the next Apply or View reports the error properly
func (engine) ToMove(s game.State) int8 {
	st, err := decode(s)
	if err != nil || st.Over() {
		return 0
	}
	return st.ToMove
}

// View is the whole board: nothing in dots and boxes is hidden, and drawnBy
// and owner stay absolute seats, so both people see the same picture
type View struct {
	Rows     int    `json:"rows"`
	Cols     int    `json:"cols"`
	Edges    int    `json:"edges"`
	Active   []bool `json:"active"`
	Drawn    []bool `json:"drawn"`
	DrawnBy  []int8 `json:"drawnBy"`
	Owner    []int8 `json:"owner"`
	ToMove   int8   `json:"toMove"`
	LastEdge *int   `json:"lastEdge"`
}

func (engine) View(s game.State, _ int8) (any, error) {
	st, err := decode(s)
	if err != nil {
		return nil, err
	}
	b := st.board()
	v := View{Rows: b.rows, Cols: b.cols, Edges: b.edges, Active: append([]bool(nil), b.active...),
		Drawn: st.Drawn, DrawnBy: st.DrawnBy, Owner: st.Owner, ToMove: st.ToMove}
	if st.LastEdge >= 0 {
		e := st.LastEdge
		v.LastEdge = &e
	}
	return v, nil
}

func (engine) Result(s game.State) game.Result {
	st, err := decode(s)
	if err != nil {
		return game.Result{}
	}
	a, b := st.Score()
	r := game.Result{Over: st.Over(), Score: [2]int{a, b}}
	if r.Over {
		r.Winner = st.Winner()
		r.Draw = r.Winner == 0
	}
	return r
}

func (engine) BotMove(s game.State, seed uint64, _ game.Options) (json.RawMessage, error) {
	st, err := decode(s)
	if err != nil {
		return nil, err
	}
	e := st.pick(seed)
	if e < 0 {
		return nil, game.ErrIllegal
	}
	return json.Marshal(move{Edge: &e})
}

// LevelInfo is one row of the level picker
type LevelInfo struct {
	Level int    `json:"level"`
	Name  string `json:"name"`
	Rows  int    `json:"rows"`
	Cols  int    `json:"cols"`
	Boxes int    `json:"boxes"` // boxes in play, after the mask
}

func (engine) Catalog() any {
	out := make([]LevelInfo, 0, MaxLevel)
	for l := 1; l <= MaxLevel; l++ {
		b := boards[l]
		out = append(out, LevelInfo{Level: l, Name: levelDefs[l].name, Rows: b.rows, Cols: b.cols, Boxes: b.activeN})
	}
	return map[string]any{"levels": out}
}
