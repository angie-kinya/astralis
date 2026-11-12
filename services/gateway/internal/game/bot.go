package game

import (
	"hash/fnv"
	"math/rand/v2"
)

// the astralis opponent. no model, no search tree: three plain rules that play
// a decent game without being ruthless. it never double-deals (declining the
// last two boxes of a chain to keep control), which is what makes strong dots
// and boxes play feel like a trap. a calm app wants a fair partner, not that
//
//  1. take any box that is one edge from closed
//  2. otherwise draw a safe edge, one that leaves no box with three sides,
//     chosen at random among the safe ones
//  3. otherwise give away as little as possible: the edge whose greedy reply
//     hands over the fewest boxes

// Seed turns a game id into the bot's seed, so a given game replays the same
// way in tests and nothing about it depends on the clock
func Seed(gameID string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(gameID))
	return h.Sum64()
}

// BotMove picks the edge for whoever is to move. it mixes how many edges are
// drawn into the seed, so consecutive moves in one game are not one draw
// repeated. -1 only for a finished board
func BotMove(s State, seed uint64) int {
	if s.Over() {
		return -1
	}
	if e, ok := s.completing(); ok {
		return e
	}

	var safe []int
	for e := range Edges {
		if !s.Drawn[e] && s.safe(e) {
			safe = append(safe, e)
		}
	}
	if len(safe) > 0 {
		rng := rand.New(rand.NewPCG(seed, uint64(s.drawnCount())))
		return safe[rng.IntN(len(safe))]
	}

	best, fewest := -1, Boxes+1
	for e := range Edges {
		if s.Drawn[e] {
			continue
		}
		if n := s.giveaway(e); n < fewest {
			best, fewest = e, n
		}
	}
	return best
}

// completing finds the lowest undrawn edge that closes some box
func (s State) completing() (int, bool) {
	for b := range Boxes {
		if s.Owner[b] != 0 || s.sides(b) != 3 {
			continue
		}
		for _, e := range edgesOf(b) {
			if !s.Drawn[e] {
				return e, true
			}
		}
	}
	return 0, false
}

// safe means drawing e takes no box to three sides, so the reply cannot capture
func (s State) safe(e int) bool {
	for _, b := range boxesOf(e) {
		if s.sides(b) >= 2 {
			return false
		}
	}
	return true
}

// giveaway counts the boxes the opponent collects by greedily closing boxes
// after e is drawn. only called when nothing is completable, so e itself
// claims nothing and the turn passes
func (s State) giveaway(e int) int {
	s.Drawn[e] = true
	n := 0
	for {
		c, ok := s.completing()
		if !ok {
			return n
		}
		s.Drawn[c] = true
		for _, b := range boxesOf(c) {
			if s.Owner[b] == 0 && s.closed(b) {
				s.Owner[b] = 3 // a placeholder owner; this copy is thrown away
				n++
			}
		}
	}
}

func (s State) sides(b int) int {
	n := 0
	for _, e := range edgesOf(b) {
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
