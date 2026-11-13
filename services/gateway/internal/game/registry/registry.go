// Package registry is the one place that knows which games exist. the store
// and the api look an engine up by games.kind and never import an engine
// package themselves, so adding a game is a new package and one line here
package registry

import (
	"sort"

	"astralis/gateway/internal/game"
	"astralis/gateway/internal/game/constellations"
)

var engines = map[string]game.Engine{}

func register(e game.Engine) { engines[e.Kind()] = e }

func init() { register(constellations.New()) }

// Get is the engine for a kind, false for one this build does not know
func Get(kind string) (game.Engine, bool) {
	e, ok := engines[kind]
	return e, ok
}

// Kinds lists every registered kind in a stable order
func Kinds() []string {
	out := make([]string, 0, len(engines))
	for k := range engines {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
