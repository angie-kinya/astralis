package agent

import (
	"math"
	"math/rand/v2"
)

// minRatingsEach is how many ratings every voice needs before thompson
// sampling takes over. below it the posteriors are too wide to mean anything
const minRatingsEach = 20

// PickVariant chooses a voice for a new session by thompson sampling:
// draw once from each voice's Beta(helpful+1, unhelpful+1) posterior and take
// the highest draw. a voice that is probably better wins more often, and a
// voice with few ratings still gets tried because its draws are spread wide
//
// stats is {helpful, unhelpful} per name. honest limit: a friends-only beta
// produces tens of ratings a week, so this separates voices over weeks; the
// eval harness is the main signal and this is confirmation from real people
func PickVariant(stats map[string][2]int, names []string, minEach int) string {
	if len(names) == 0 {
		return DefaultVariant
	}
	// explore uniformly until every voice has enough ratings to mean anything
	for _, n := range names {
		if s := stats[n]; s[0]+s[1] < minEach {
			return names[rand.IntN(len(names))]
		}
	}
	best, bestDraw := names[0], -1.0
	for _, n := range names {
		s := stats[n]
		if d := betaDraw(float64(s[0]+1), float64(s[1]+1)); d > bestDraw {
			best, bestDraw = n, d
		}
	}
	return best
}

// betaDraw samples Beta(a, b) as X/(X+Y) with X~Gamma(a), Y~Gamma(b).
// two gamma draws beat pulling in a numerics library for one distribution
func betaDraw(a, b float64) float64 {
	x, y := gammaDraw(a), gammaDraw(b)
	return x / (x + y)
}

// gammaDraw samples Gamma(k, 1) by marsaglia and tsang (2000). for k < 1 it
// draws Gamma(k+1) and scales by U^(1/k), the standard boost
func gammaDraw(k float64) float64 {
	if k < 1 {
		return gammaDraw(k+1) * math.Pow(rand.Float64(), 1/k)
	}
	d := k - 1.0/3
	c := 1 / math.Sqrt(9*d)
	for {
		x := rand.NormFloat64()
		v := 1 + c*x
		if v <= 0 {
			continue
		}
		v = v * v * v
		u := rand.Float64()
		if u < 1-0.0331*x*x*x*x || math.Log(u) < 0.5*x*x+d*(1-v+math.Log(v)) {
			return d * v
		}
	}
}
