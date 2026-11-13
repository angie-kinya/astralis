// Package words is a calm word puzzle in the spirit of ather: guess a hidden
// word, each guess marked letter by letter. it is server-authoritative, so
// the target lives only in the stored state and never reaches a view until
// the game is over
//
// two seats make a duel: both solve the same word on their own, neither sees
// the other's letters, and the results are compared once both are done. there
// is no turn order, no clock and no dictionary check: any letters of the right
// length are a fair guess, because being told "not a word" is friction, not fun
package words

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"

	"astralis/gateway/internal/game"
)

type entry struct {
	Word       string
	Difficulty int // 1-5, as in ather; kept for a later picker, not read yet
	Clue       string
}

type bank struct {
	id, name, blurb string
	words           []entry
	facts           []string
}

type mode struct {
	id, name, blurb string
	// guesses by word length, ather's PUZZLE_MODES tables
	byLen map[int]int
}

// speed and challenge are left out on purpose: a clock is pressure, and
// "fewer guesses for more points" is the opposite of a quiet game
var modes = []mode{
	{"classic", "classic", "guess the word, letter by letter", map[int]int{4: 5, 5: 6, 6: 6, 7: 7, 8: 8, 9: 8}},
	{"anagram", "anagram", "the letters are all there, just shuffled", map[int]int{4: 4, 5: 4, 6: 5, 7: 5, 8: 6}},
	{"mystery", "mystery", "the topic stays hidden until the end", map[int]int{4: 5, 5: 6, 6: 7, 7: 8, 8: 9}},
}

func findMode(id string) (mode, bool) {
	for _, m := range modes {
		if m.id == id {
			return m, true
		}
	}
	return mode{}, false
}

func findBank(id string) (bank, bool) {
	for _, b := range banks {
		if b.id == id {
			return b, true
		}
	}
	return bank{}, false
}

// maxGuesses mirrors ather's get_mode_max_guesses: the exact length if the
// table has it, else the nearest shorter length, else the shortest
func (m mode) maxGuesses(n int) int {
	if g, ok := m.byLen[n]; ok {
		return g
	}
	best, shortest := 0, 0
	for l := range m.byLen {
		if l <= n && l > best {
			best = l
		}
		if shortest == 0 || l < shortest {
			shortest = l
		}
	}
	if best > 0 {
		return m.byLen[best]
	}
	return m.byLen[shortest]
}

// Mark is one letter's verdict
type Mark string

const (
	Correct Mark = "correct"
	Present Mark = "present"
	Absent  Mark = "absent"
)

// Evaluate marks guess against target, both upper case and the same length.
// it is ather's evaluate_guess: exact matches are claimed first, then each
// remaining guess letter takes one unclaimed copy from the target, left to
// right, so a letter is never marked present more times than the target can
// pay for
func Evaluate(target, guess string) []Mark {
	marks := make([]Mark, len(guess))
	var left [26]int
	for i := 0; i < len(target); i++ {
		if guess[i] == target[i] {
			marks[i] = Correct
		} else {
			left[target[i]-'A']++
		}
	}
	for i := 0; i < len(guess); i++ {
		if marks[i] == Correct {
			continue
		}
		if c := guess[i] - 'A'; left[c] > 0 {
			marks[i] = Present
			left[c]--
		} else {
			marks[i] = Absent
		}
	}
	return marks
}

// maxHints is the number of hint tiers: clue, one letter, first and last
const maxHints = 3

type seat struct {
	Guesses []string `json:"guesses"`
	Hints   []string `json:"hints"`
	Solved  bool     `json:"solved"`
}

// state is what games.state holds. Seed is kept so the letter hint is a
// pure function of the stored game rather than of when it was asked for
type state struct {
	Seed       uint64 `json:"seed"`
	Topic      string `json:"topic"`
	Mode       string `json:"mode"`
	Word       string `json:"word"`
	Clue       string `json:"clue"`
	Fact       string `json:"fact"`
	Scrambled  string `json:"scrambled,omitempty"`
	MaxGuesses int    `json:"maxGuesses"`
	Seats      []seat `json:"seats"`
}

func (s *state) done(i int) bool {
	return s.Seats[i].Solved || len(s.Seats[i].Guesses) >= s.MaxGuesses
}

func (s *state) over() bool {
	for i := range s.Seats {
		if !s.done(i) {
			return false
		}
	}
	return true
}

func upperLetters(w string) bool {
	for i := 0; i < len(w); i++ {
		if w[i] < 'A' || w[i] > 'Z' {
			return false
		}
	}
	return w != ""
}

// decode refuses any stored state the rules could not have produced, so a
// corrupt row is an error rather than an index panic later
func decode(raw game.State) (*state, error) {
	var s state
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	m, ok := findMode(s.Mode)
	n := len(s.Word)
	switch {
	case !ok:
		return nil, errors.New("words state: unknown mode")
	case n < 4 || n > 9 || !upperLetters(s.Word):
		return nil, errors.New("words state: bad word")
	case s.MaxGuesses != m.maxGuesses(n):
		return nil, errors.New("words state: bad maxGuesses")
	case len(s.Seats) != 1 && len(s.Seats) != 2:
		return nil, errors.New("words state: bad seats")
	case s.Mode == "anagram" && len(s.Scrambled) != n:
		return nil, errors.New("words state: bad scramble")
	}
	if _, ok := findBank(s.Topic); !ok {
		return nil, errors.New("words state: unknown topic")
	}
	for _, st := range s.Seats {
		if len(st.Guesses) > s.MaxGuesses || len(st.Hints) > maxHints {
			return nil, errors.New("words state: too many moves")
		}
		for _, g := range st.Guesses {
			if len(g) != n || !upperLetters(g) {
				return nil, errors.New("words state: bad guess")
			}
		}
		last := len(st.Guesses) > 0 && st.Guesses[len(st.Guesses)-1] == s.Word
		if st.Solved != last {
			return nil, errors.New("words state: solved out of step")
		}
	}
	return &s, nil
}

// Engine is the words game. it has no fields: everything lives in the state
type Engine struct{}

// New returns the words engine for the registry
func New() game.Engine { return Engine{} }

func (Engine) Kind() string { return "words" }
func (Engine) Seats() int   { return 2 }

// New picks the word. an empty mode is classic. mystery may come without a
// topic (the web sends none), and then the seed picks the bank; every other
// mode needs one. the same seed and options always give the same word, which
// is all a daily puzzle needs. error text is shown to people as it is, so it
// is plain and kind
func (Engine) New(seed uint64, o game.Options, players int) (game.State, error) {
	if players != 1 && players != 2 {
		return nil, errors.New("a word puzzle is for one person, or two in a duel")
	}
	if o.Mode == "" {
		o.Mode = "classic"
	}
	m, ok := findMode(o.Mode)
	if !ok {
		return nil, errors.New("that way of playing isn't one we know yet: try classic, anagram or mystery")
	}
	// a fixed stream id keeps words apart from any other engine fed the same seed
	rng := rand.New(rand.NewPCG(seed, 0x776f726473))
	var b bank
	switch {
	case o.Topic == "" && m.id == "mystery":
		b = banks[rng.IntN(len(banks))]
	case o.Topic == "":
		return nil, errors.New("pick a topic to begin")
	default:
		if b, ok = findBank(o.Topic); !ok {
			return nil, errors.New("we couldn't find that topic: try picking another")
		}
	}
	e := b.words[rng.IntN(len(b.words))]
	s := state{
		Seed:       seed,
		Topic:      b.id,
		Mode:       m.id,
		Word:       e.Word,
		Clue:       e.Clue,
		Fact:       b.facts[rng.IntN(len(b.facts))],
		MaxGuesses: m.maxGuesses(len(e.Word)),
		Seats:      make([]seat, players),
	}
	if m.id == "anagram" {
		s.Scrambled = scramble(e.Word, rng)
	}
	for i := range s.Seats {
		s.Seats[i] = seat{Guesses: []string{}, Hints: []string{}}
	}
	return json.Marshal(s)
}

// scramble shuffles until the letters differ from the word. every bank word
// has at least two distinct letters, so the rotation fallback is only there
// to make termination obvious
func scramble(w string, rng *rand.Rand) string {
	b := []byte(w)
	for range 10 {
		rng.Shuffle(len(b), func(i, j int) { b[i], b[j] = b[j], b[i] })
		if string(b) != w {
			return string(b)
		}
	}
	return w[1:] + w[:1]
}

type move struct {
	Guess *string `json:"guess"`
	Hint  bool    `json:"hint"`
}

// Apply takes {"guess":"WORD"} or {"hint":true} from seat 1 or 2. a seat that
// is done (solved, or out of guesses) can no longer move, though the other
// seat in a duel plays on
func (Engine) Apply(raw game.State, who int8, mv json.RawMessage) (game.State, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	i := int(who) - 1
	if i < 0 || i >= len(s.Seats) || s.done(i) {
		return nil, game.ErrIllegal
	}
	var m move
	if err := json.Unmarshal(mv, &m); err != nil {
		return nil, game.ErrIllegal
	}
	st := &s.Seats[i]
	switch {
	case m.Guess != nil && !m.Hint:
		g := strings.ToUpper(strings.TrimSpace(*m.Guess))
		if len(g) != len(s.Word) || !upperLetters(g) {
			return nil, game.ErrIllegal
		}
		st.Guesses = append(st.Guesses, g)
		st.Solved = g == s.Word
	case m.Hint && m.Guess == nil:
		if len(st.Hints) >= maxHints {
			return nil, game.ErrIllegal
		}
		st.Hints = append(st.Hints, s.hint(i))
	default:
		return nil, game.ErrIllegal
	}
	return json.Marshal(s)
}

// hint is the next tier for seat i: the clue, then one letter the seat has
// not already placed (preferring the middle, since the last tier gives the
// ends), then the first and last letters. hints are free: they cost nothing
// and are never counted against anyone
func (s *state) hint(i int) string {
	n := len(s.Word)
	switch len(s.Seats[i].Hints) {
	case 0:
		return s.Clue
	case 1:
		var known [9]bool
		for _, g := range s.Seats[i].Guesses {
			for p := range n {
				known[p] = known[p] || g[p] == s.Word[p]
			}
		}
		var middle, any []int
		for p := range n {
			if !known[p] {
				any = append(any, p)
				if p > 0 && p < n-1 {
					middle = append(middle, p)
				}
			}
		}
		pool := middle
		if len(pool) == 0 {
			pool = any
		}
		if len(pool) == 0 {
			return "you have already placed every letter"
		}
		rng := rand.New(rand.NewPCG(s.Seed, uint64(i+1)))
		p := pool[rng.IntN(len(pool))]
		return fmt.Sprintf("letter %d is %c", p+1, s.Word[p])
	default:
		return fmt.Sprintf("it starts with %c and ends with %c", s.Word[0], s.Word[n-1])
	}
}

// ToMove is always 0: a duel has no turns, and a solo puzzle has one player
func (Engine) ToMove(game.State) int8 { return 0 }

// Guess is one guess with its marks
type Guess struct {
	Word  string `json:"word"`
	Marks []Mark `json:"marks"`
}

// Opponent is all a duel shows of the other seat: progress, never letters
type Opponent struct {
	Guesses int  `json:"guesses"`
	Solved  bool `json:"solved"`
	Over    bool `json:"over"`
}

// View is WordsView in the contract. Over is this seat's own: it has solved
// or run out of guesses. Answer, Clue and Fact (and Topic in mystery) stay nil
// until the whole game is over, so in a duel the first to finish cannot pass
// the word to the other
type View struct {
	Length     int             `json:"length"`
	MaxGuesses int             `json:"maxGuesses"`
	Mode       string          `json:"mode"`
	Topic      *string         `json:"topic"`
	Scrambled  *string         `json:"scrambled"`
	Guesses    []Guess         `json:"guesses"`
	Letters    map[string]Mark `json:"letters"`
	Hints      []string        `json:"hints"`
	HintsLeft  int             `json:"hintsLeft"`
	Solved     bool            `json:"solved"`
	Over       bool            `json:"over"`
	Answer     *string         `json:"answer"`
	Clue       *string         `json:"clue"`
	Fact       *string         `json:"fact"`
	Opponent   *Opponent       `json:"opponent"`
}

var rank = map[Mark]int{Absent: 1, Present: 2, Correct: 3}

func (Engine) View(raw game.State, who int8) (any, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	i := int(who) - 1
	if i < 0 || i >= len(s.Seats) {
		return nil, fmt.Errorf("words: no seat %d", who)
	}
	st := s.Seats[i]
	v := View{
		Length:     len(s.Word),
		MaxGuesses: s.MaxGuesses,
		Mode:       s.Mode,
		Guesses:    make([]Guess, 0, len(st.Guesses)),
		Letters:    map[string]Mark{},
		Hints:      append([]string{}, st.Hints...),
		HintsLeft:  maxHints - len(st.Hints),
		Solved:     st.Solved,
		Over:       s.done(i),
	}
	for _, g := range st.Guesses {
		marks := Evaluate(s.Word, g)
		v.Guesses = append(v.Guesses, Guess{Word: g, Marks: marks})
		// a letter shows its best verdict so far: correct beats present beats absent
		for p, mk := range marks {
			l := g[p : p+1]
			if rank[mk] > rank[v.Letters[l]] {
				v.Letters[l] = mk
			}
		}
	}
	over := s.over()
	if s.Mode != "mystery" || over {
		v.Topic = &s.Topic
	}
	if s.Scrambled != "" {
		v.Scrambled = &s.Scrambled
	}
	if over {
		v.Answer, v.Clue, v.Fact = &s.Word, &s.Clue, &s.Fact
	}
	if len(s.Seats) == 2 {
		o := s.Seats[1-i]
		v.Opponent = &Opponent{Guesses: len(o.Guesses), Solved: o.Solved, Over: s.done(1 - i)}
	}
	return v, nil
}

// Result: Score is guesses used per seat, lower is better. in a duel the seat
// that solved in fewer guesses wins, and an unsolved seat never beats a
// solved one; equal guesses or both unsolved is a draw. Solved is the solo
// player's outcome, or in a duel whether anyone found the word (each seat's
// own outcome is in its view)
func (Engine) Result(raw game.State) game.Result {
	s, err := decode(raw)
	if err != nil {
		return game.Result{}
	}
	var r game.Result
	for i, st := range s.Seats {
		r.Score[i] = len(st.Guesses)
		r.Solved = r.Solved || st.Solved
	}
	if !s.over() {
		return game.Result{Score: r.Score}
	}
	r.Over = true
	if len(s.Seats) == 1 {
		return r
	}
	a, b := s.Seats[0], s.Seats[1]
	switch {
	case a.Solved && (!b.Solved || len(a.Guesses) < len(b.Guesses)):
		r.Winner = 1
	case b.Solved && (!a.Solved || len(b.Guesses) < len(a.Guesses)):
		r.Winner = 2
	default:
		r.Draw = true
	}
	return r
}

// Topic and ModeInfo are the catalog entries the ui lists
type Topic struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Blurb string `json:"blurb"`
}

type ModeInfo = Topic

// Catalog is {topics:[...], modes:[...]}, served as-is by GET /games/catalog
func (Engine) Catalog() any {
	c := struct {
		Topics []Topic    `json:"topics"`
		Modes  []ModeInfo `json:"modes"`
	}{}
	for _, b := range banks {
		c.Topics = append(c.Topics, Topic{b.id, b.name, b.blurb})
	}
	for _, m := range modes {
		c.Modes = append(c.Modes, ModeInfo{m.id, m.name, m.blurb})
	}
	return c
}

var (
	_ game.Engine     = Engine{}
	_ game.Cataloguer = Engine{}
)
