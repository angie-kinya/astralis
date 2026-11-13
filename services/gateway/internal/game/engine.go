// Package game holds the contract every game in astralis implements. the
// engines themselves live in subpackages (constellations, words, sudoku);
// the store and the api only ever see this interface, so a new game is a new
// package and one registry line, never a schema or handler change
//
// state is opaque json on purpose: it is persisted as-is in games.state
// (jsonb), and only the engine that wrote it knows its shape
package game

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"strconv"
	"time"
)

// State is an engine's whole game state, exactly as stored in games.state
type State = json.RawMessage

// ErrIllegal is returned for any move the rules refuse. the state passed in
// is never modified, so a caller can always fall back to what it had
var ErrIllegal = errors.New("illegal move")

// Options pick the variant of a game. each engine reads only its own fields
// and must validate them in New (an unknown level or topic is an error)
type Options struct {
	Level int    `json:"level,omitempty"` // constellations 1-5, sudoku 1-4 (mini6, easy, medium, hard)
	Topic string `json:"topic,omitempty"` // words: a bank id ("astronomy", "calm", ...)
	Mode  string `json:"mode,omitempty"`  // words: classic | anagram | mystery
	Day   string `json:"day,omitempty"`   // set for a daily puzzle: the nairobi date, YYYY-MM-DD
}

// Result is how a game stands. Over is the only field that matters while it
// is still being played
type Result struct {
	Over   bool
	Winner int8   // 0 when nobody won: a draw, a solo puzzle, or not over yet
	Draw   bool   // two seats, over, level on score
	Score  [2]int // seat 1, seat 2: boxes, or for words fewer guesses is better (see words)
	Solved bool   // puzzles: the player reached the answer
}

// Engine is one game's rules. implementations are pure: no clock, no
// randomness beyond the seed, no io. that is what makes them fuzzable and
// lets a daily puzzle be the same for everyone
type Engine interface {
	Kind() string
	// Seats is 2 for games two people can play against each other, 1 for
	// puzzles that only ever have one player
	Seats() int
	// New builds the starting state. players is 1 (solo, or against astralis)
	// or 2 (peer). the same seed and options always give the same game
	New(seed uint64, o Options, players int) (State, error)
	// Apply plays one move for seat 1 or 2. the move shape is the engine's
	// own ({"edge":3}, {"guess":"ORBIT"}, {"grid":"..."})
	Apply(s State, seat int8, move json.RawMessage) (State, error)
	// ToMove is the seat whose turn it is, or 0 when either seat may move
	// (puzzles, words duels) or the game is over
	ToMove(s State) int8
	// View is what one seat may see. it must never include a secret (the
	// target word, the sudoku solution) before the game is over
	View(s State, seat int8) (any, error)
	Result(s State) Result
}

// Bot is implemented by engines astralis can play against
type Bot interface {
	// BotMove picks astralis's next move. seed is per game, so a replay of
	// the same game makes the same choices
	BotMove(s State, seed uint64, o Options) (json.RawMessage, error)
}

// Cataloguer is implemented by engines with choices the ui must list
// (levels, topics, modes). the result is served as-is by GET /games/catalog
type Cataloguer interface {
	Catalog() any
}

// nairobi is where astralis's day turns over, for daily puzzles and the
// weekly board alike. a fixed offset is exact: kenya has no daylight saving
var nairobi = time.FixedZone("EAT", 3*60*60)

// Today is the current daily-puzzle date, YYYY-MM-DD in nairobi
func Today(now time.Time) string { return now.In(nairobi).Format("2006-01-02") }

// DailySeed makes everyone's daily puzzle identical for a given kind, topic,
// mode and level, and different the next day
func DailySeed(kind string, o Options) uint64 {
	h := sha256.Sum256([]byte(kind + "|" + o.Day + "|" + o.Topic + "|" + o.Mode + "|" + strconv.Itoa(o.Level)))
	return binary.BigEndian.Uint64(h[:8])
}
