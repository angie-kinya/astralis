package constellations

import "math/rand/v2"

// the astralis opponent. no model, no search tree: three plain rules that play
// a decent game without being ruthless. it never double-deals (declining the
// last two boxes of a chain to keep control), which is what makes strong dots
// and boxes play feel like a trap. a calm app wants a fair partner, not that
//
//  1. take any box that is one edge from closed (on gentle levels, now and
//     then leave it and play a safe edge instead, so the box stays there for
//     the person to take)
//  2. otherwise draw a safe edge, one that leaves no box with three sides,
//     chosen at random among the safe ones
//  3. otherwise give away as little as possible: the edge whose greedy reply
//     hands over the fewest boxes

// gentleUpTo is the highest level where the bot sometimes passes up a box
const gentleUpTo = 2

// gentleOdds is one in how many boxes the bot leaves on a gentle level
const gentleOdds = 3

// pick returns the edge for whoever is to move. it mixes how many edges are
// drawn into the seed, so consecutive moves in one game are not one draw
// repeated. -1 only for a finished board
func (s State) pick(seed uint64) int {
	if s.Over() {
		return -1
	}
	drawn := uint64(s.drawnCount())
	safe := s.safeEdges()

	if e, ok := s.completing(); ok {
		// a separate stream from the safe choice below, so being gentle does not
		// also steer which safe edge comes next
		kind := s.Level <= gentleUpTo && len(safe) > 0 &&
			rand.New(rand.NewPCG(seed, drawn<<1|1)).IntN(gentleOdds) == 0
		if !kind {
			return e
		}
	}
	if len(safe) > 0 {
		rng := rand.New(rand.NewPCG(seed, drawn))
		return safe[rng.IntN(len(safe))]
	}

	b := s.board()
	best, fewest := -1, b.boxes+1
	for e := range b.edges {
		if !b.playable[e] || s.Drawn[e] {
			continue
		}
		if n := s.giveaway(e); n < fewest {
			best, fewest = e, n
		}
	}
	return best
}

func (s State) safeEdges() []int {
	b := s.board()
	var out []int
	for e := range b.edges {
		if b.playable[e] && !s.Drawn[e] && s.safe(e) {
			out = append(out, e)
		}
	}
	return out
}

// completing finds the lowest undrawn edge that closes some box
func (s State) completing() (int, bool) {
	b := s.board()
	for box := range b.boxes {
		if !b.active[box] || s.Owner[box] != 0 || s.sides(box) != 3 {
			continue
		}
		for _, e := range b.boxEdges[box] {
			if !s.Drawn[e] {
				return e, true
			}
		}
	}
	return 0, false
}

// safe means drawing e takes no box to three sides, so the reply cannot capture
func (s State) safe(e int) bool {
	for _, box := range s.board().edgeBoxes[e] {
		if s.sides(box) >= 2 {
			return false
		}
	}
	return true
}

// giveaway counts the boxes the opponent collects by greedily closing boxes
// after e is drawn. only called when nothing is completable, so e itself
// claims nothing and the turn passes
func (s State) giveaway(e int) int {
	s = s.clone()
	b := s.board()
	s.Drawn[e] = true
	n := 0
	for {
		c, ok := s.completing()
		if !ok {
			return n
		}
		s.Drawn[c] = true
		for _, box := range b.edgeBoxes[c] {
			if s.Owner[box] == 0 && s.closed(box) {
				s.Owner[box] = 3 // a placeholder owner; this copy is thrown away
				n++
			}
		}
	}
}
