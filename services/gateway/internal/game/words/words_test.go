package words

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"astralis/gateway/internal/game"
)

// expected marks come from running ather's evaluate_guess on the same pairs:
// C correct, P present, . absent
func TestEvaluateMatchesAther(t *testing.T) {
	cases := []struct{ target, guess, want string }{
		{"ABBEY", "BABES", "PPCC."},
		{"KEBAB", "BOBBY", "P.C.."},
		{"BOBBY", "KEBAB", "..C.P"},
		{"ABBEY", "KEBAB", ".PCPP"},
		{"ABBEY", "ABYSS", "CCP.."},
		{"SPEED", "ERASE", "P..PP"},
		{"CRANE", "EERIE", "..P.C"},
		{"LLAMA", "HALLS", ".PPP."},
		{"ROBOT", "OOOOO", ".C.C."},
		{"ASTEROID", "STEROIDS", "PPPPPPP."},
		{"CODE", "CODE", "CCCC"},
		{"NOVA", "AAAA", "...C"},
		{"APPLE", "PAPAL", "PPC.P"},
		{"ELDER", "EEEEE", "C..C."},
	}
	short := map[Mark]byte{Correct: 'C', Present: 'P', Absent: '.'}
	for _, c := range cases {
		var got strings.Builder
		for _, m := range Evaluate(c.target, c.guess) {
			got.WriteByte(short[m])
		}
		if got.String() != c.want {
			t.Errorf("Evaluate(%s, %s) = %s, want %s", c.target, c.guess, got.String(), c.want)
		}
	}
}

func TestMaxGuessesFollowsAtherTables(t *testing.T) {
	cases := []struct {
		mode string
		n    int
		want int
	}{
		{"classic", 4, 5}, {"classic", 5, 6}, {"classic", 7, 7}, {"classic", 9, 8},
		{"anagram", 4, 4}, {"anagram", 8, 6}, {"anagram", 9, 6},
		{"mystery", 6, 7}, {"mystery", 8, 9}, {"mystery", 9, 9},
	}
	for _, c := range cases {
		m, _ := findMode(c.mode)
		if got := m.maxGuesses(c.n); got != c.want {
			t.Errorf("%s len %d: %d guesses, want %d", c.mode, c.n, got, c.want)
		}
	}
}

func TestBanksAreClean(t *testing.T) {
	for _, b := range banks {
		if len(b.words) == 0 || len(b.facts) == 0 {
			t.Fatalf("%s: empty bank", b.id)
		}
		seen := map[string]bool{}
		for _, e := range b.words {
			if n := len(e.Word); n < 4 || n > 9 || !upperLetters(e.Word) {
				t.Errorf("%s: bad word %q", b.id, e.Word)
			}
			if seen[e.Word] {
				t.Errorf("%s: duplicate %s", b.id, e.Word)
			}
			seen[e.Word] = true
			if e.Difficulty < 1 || e.Difficulty > 5 || e.Clue == "" {
				t.Errorf("%s: %s needs a difficulty 1-5 and a clue", b.id, e.Word)
			}
			// the first hint is the clue, so it must not simply be the answer
			if strings.Contains(strings.ToUpper(e.Clue), e.Word) {
				t.Errorf("%s: clue for %s contains it", b.id, e.Word)
			}
		}
		for _, f := range b.facts {
			if strings.ContainsAny(f, "—–") {
				t.Errorf("%s: dash in fact %q", b.id, f)
			}
		}
	}
}

func mustNew(t testing.TB, seed uint64, o game.Options, players int) game.State {
	t.Helper()
	s, err := Engine{}.New(seed, o, players)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func play(t testing.TB, s game.State, seat int8, mv string) game.State {
	t.Helper()
	next, err := Engine{}.Apply(s, seat, json.RawMessage(mv))
	if err != nil {
		t.Fatalf("move %s: %v", mv, err)
	}
	return next
}

func secret(t testing.TB, s game.State) *state {
	t.Helper()
	st, err := decode(s)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func view(t testing.TB, s game.State, seat int8) View {
	t.Helper()
	v, err := Engine{}.View(s, seat)
	if err != nil {
		t.Fatal(err)
	}
	return v.(View)
}

func TestNewValidatesOptions(t *testing.T) {
	bad := []struct {
		o       game.Options
		players int
	}{
		{game.Options{Topic: "cooking"}, 1},
		{game.Options{Mode: "speed"}, 1},
		{game.Options{Topic: "calm"}, 0},
		{game.Options{Topic: "calm"}, 3},
		{game.Options{}, 1},
		{game.Options{Mode: "classic"}, 1},
		{game.Options{Mode: "anagram"}, 1},
	}
	for _, c := range bad {
		if _, err := (Engine{}).New(1, c.o, c.players); err == nil {
			t.Errorf("New(%+v, %d) accepted", c.o, c.players)
		}
	}
	for _, b := range banks {
		for _, m := range modes {
			mustNew(t, 7, game.Options{Topic: b.id, Mode: m.id}, 2)
		}
	}
}

func TestSameSeedSameWord(t *testing.T) {
	o := game.Options{Topic: "calm", Mode: "anagram", Day: "2026-11-13"}
	seed := game.DailySeed("words", o)
	a, b := mustNew(t, seed, o, 1), mustNew(t, seed, o, 1)
	if string(a) != string(b) {
		t.Fatal("same seed gave different games")
	}
	words := map[string]bool{}
	for s := range uint64(40) {
		words[secret(t, mustNew(t, s, o, 1)).Word] = true
	}
	if len(words) < 5 {
		t.Fatalf("40 seeds gave only %d words", len(words))
	}
}

func TestAnagramIsScrambled(t *testing.T) {
	for seed := range uint64(200) {
		st := secret(t, mustNew(t, seed, game.Options{Topic: "astronomy", Mode: "anagram"}, 1))
		if st.Scrambled == st.Word || sorted(st.Scrambled) != sorted(st.Word) {
			t.Fatalf("seed %d: %s is not a scramble of %s", seed, st.Scrambled, st.Word)
		}
	}
}

func sorted(w string) string {
	b := []byte(w)
	for i := range b {
		for j := i + 1; j < len(b); j++ {
			if b[j] < b[i] {
				b[i], b[j] = b[j], b[i]
			}
		}
	}
	return string(b)
}

func TestSoloSolveAndMiss(t *testing.T) {
	s := mustNew(t, 3, game.Options{Topic: "calm"}, 1)
	word := secret(t, s).Word
	wrong := strings.Repeat("Q", len(word))

	s = play(t, s, 1, fmt.Sprintf(`{"guess":%q}`, strings.ToLower(wrong)))
	if r := (Engine{}).Result(s); r.Over {
		t.Fatal("over after one miss")
	}
	v := view(t, s, 1)
	if v.Answer != nil || v.Fact != nil || v.Clue != nil || v.Letters["Q"] != Absent {
		t.Fatalf("active view: %+v", v)
	}
	s = play(t, s, 1, fmt.Sprintf(`{"guess":%q}`, word))
	r := Engine{}.Result(s)
	if !r.Over || !r.Solved || r.Score[0] != 2 || r.Winner != 0 {
		t.Fatalf("result after solve: %+v", r)
	}
	if v := view(t, s, 1); v.Answer == nil || *v.Answer != word || !v.Over || !v.Solved {
		t.Fatalf("finished view: %+v", v)
	}
	if _, err := (Engine{}).Apply(s, 1, json.RawMessage(`{"guess":"`+word+`"}`)); !errors.Is(err, game.ErrIllegal) {
		t.Fatal("moved after solving")
	}

	s = mustNew(t, 3, game.Options{Topic: "calm"}, 1)
	for range secret(t, s).MaxGuesses {
		s = play(t, s, 1, `{"guess":"`+wrong+`"}`)
	}
	if r := (Engine{}).Result(s); !r.Over || r.Solved {
		t.Fatalf("out of guesses: %+v", r)
	}
}

func TestIllegalMoves(t *testing.T) {
	s := mustNew(t, 9, game.Options{Topic: "tech"}, 1)
	n := len(secret(t, s).Word)
	for _, mv := range []string{
		`{}`, `nope`, `{"guess":""}`, `{"guess":"` + strings.Repeat("A", n+1) + `"}`,
		`{"guess":"` + strings.Repeat("1", n) + `"}`, `{"guess":"` + strings.Repeat("É", n/2) + `"}`,
		`{"guess":"ABCD","hint":true}`, `{"hint":false}`,
	} {
		if _, err := (Engine{}).Apply(s, 1, json.RawMessage(mv)); !errors.Is(err, game.ErrIllegal) {
			t.Errorf("%s: err %v, want ErrIllegal", mv, err)
		}
	}
	if _, err := (Engine{}).Apply(s, 2, json.RawMessage(`{"hint":true}`)); !errors.Is(err, game.ErrIllegal) {
		t.Error("seat 2 moved in a solo game")
	}
}

func TestHintTiers(t *testing.T) {
	s := mustNew(t, 11, game.Options{Topic: "mythology"}, 1)
	st := secret(t, s)
	for range maxHints {
		s = play(t, s, 1, `{"hint":true}`)
	}
	v := view(t, s, 1)
	if v.HintsLeft != 0 || len(v.Hints) != 3 {
		t.Fatalf("hints %v left %d", v.Hints, v.HintsLeft)
	}
	if v.Hints[0] != st.Clue {
		t.Errorf("first hint %q, want the clue", v.Hints[0])
	}
	var p int
	var c byte
	if _, err := fmt.Sscanf(v.Hints[1], "letter %d is %c", &p, &c); err != nil || st.Word[p-1] != c {
		t.Errorf("second hint %q does not match %s", v.Hints[1], st.Word)
	}
	if p == 1 || p == len(st.Word) {
		t.Errorf("letter hint gave an end letter, which the third hint gives anyway")
	}
	want := fmt.Sprintf("it starts with %c and ends with %c", st.Word[0], st.Word[len(st.Word)-1])
	if v.Hints[2] != want {
		t.Errorf("third hint %q, want %q", v.Hints[2], want)
	}
	if _, err := (Engine{}).Apply(s, 1, json.RawMessage(`{"hint":true}`)); !errors.Is(err, game.ErrIllegal) {
		t.Error("a fourth hint was given")
	}
	if r := (Engine{}).Result(s); r.Over || r.Score[0] != 0 {
		t.Errorf("hints should cost nothing: %+v", r)
	}
}

func TestLetterHintSkipsPlacedLetters(t *testing.T) {
	s := mustNew(t, 5, game.Options{Topic: "calm"}, 1)
	w := secret(t, s).Word
	// place every middle letter, so only an end is left to give
	guess := []byte(strings.Repeat("Q", len(w)))
	copy(guess[1:len(w)-1], w[1:len(w)-1])
	s = play(t, s, 1, `{"guess":"`+string(guess)+`"}`)
	s = play(t, s, 1, `{"hint":true}`)
	s = play(t, s, 1, `{"hint":true}`)
	h := view(t, s, 1).Hints[1]
	if h != fmt.Sprintf("letter 1 is %c", w[0]) && h != fmt.Sprintf("letter %d is %c", len(w), w[len(w)-1]) {
		t.Fatalf("hint %q for %s gave a letter already placed", h, w)
	}
}

func TestMysteryHidesTopic(t *testing.T) {
	topics := map[string]bool{}
	for seed := range uint64(60) {
		topics[secret(t, mustNew(t, seed, game.Options{Mode: "mystery"}, 1)).Topic] = true
	}
	if len(topics) != len(banks) {
		t.Fatalf("mystery without a topic reached only %d banks", len(topics))
	}
	s := mustNew(t, 2, game.Options{Mode: "mystery"}, 2)
	if v := view(t, s, 1); v.Topic != nil {
		t.Fatalf("topic shown: %s", *v.Topic)
	}
	s = play(t, s, 1, `{"guess":"`+secret(t, s).Word+`"}`)
	if v := view(t, s, 1); v.Topic != nil {
		t.Fatal("topic shown while the other seat still plays")
	}
	s = play(t, s, 2, `{"guess":"`+secret(t, s).Word+`"}`)
	if v := view(t, s, 1); v.Topic == nil || *v.Topic != secret(t, s).Topic {
		t.Fatal("topic still hidden once over")
	}
}

func TestDuel(t *testing.T) {
	fresh := func() (game.State, string, string) {
		s := mustNew(t, 21, game.Options{Topic: "calm", Mode: "classic"}, 2)
		w := secret(t, s).Word
		return s, w, strings.Repeat("Z", len(w))
	}
	solve := func(s game.State, seat int8, misses int, w, miss string) game.State {
		for range misses {
			s = play(t, s, seat, `{"guess":"`+miss+`"}`)
		}
		return play(t, s, seat, `{"guess":"`+w+`"}`)
	}
	cases := []struct {
		name   string
		run    func(s game.State, w, miss string) game.State
		winner int8
		draw   bool
		score  [2]int
	}{
		{"fewer guesses wins", func(s game.State, w, m string) game.State {
			return solve(solve(s, 1, 2, w, m), 2, 0, w, m)
		}, 2, false, [2]int{3, 1}},
		{"equal is a draw", func(s game.State, w, m string) game.State {
			return solve(solve(s, 2, 1, w, m), 1, 1, w, m)
		}, 0, true, [2]int{2, 2}},
		{"solved beats unsolved", func(s game.State, w, m string) game.State {
			s = solve(s, 2, secret(t, s).MaxGuesses-1, w, m)
			for range secret(t, s).MaxGuesses {
				s = play(t, s, 1, `{"guess":"`+m+`"}`)
			}
			return s
		}, 2, false, [2]int{0, 0}},
		{"both unsolved is a draw", func(s game.State, w, m string) game.State {
			for range secret(t, s).MaxGuesses {
				s = play(t, s, 1, `{"guess":"`+m+`"}`)
				s = play(t, s, 2, `{"guess":"`+m+`"}`)
			}
			return s
		}, 0, true, [2]int{0, 0}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, w, miss := fresh()
			s = c.run(s, w, miss)
			r := Engine{}.Result(s)
			if !r.Over || r.Winner != c.winner || r.Draw != c.draw {
				t.Fatalf("result %+v", r)
			}
			if c.score != [2]int{} && r.Score != c.score {
				t.Fatalf("score %v, want %v", r.Score, c.score)
			}
		})
	}

	// while one seat is done and the other plays on, nobody sees the answer
	s, w, miss := fresh()
	s = play(t, s, 1, `{"guess":"`+w+`"}`)
	s = play(t, s, 2, `{"guess":"`+miss+`"}`)
	if r := (Engine{}).Result(s); r.Over {
		t.Fatal("duel over with one seat still playing")
	}
	v1, v2 := view(t, s, 1), view(t, s, 2)
	if !v1.Over || v1.Answer != nil || v2.Answer != nil || v2.Over {
		t.Fatalf("half-finished duel: %+v / %+v", v1, v2)
	}
	if v2.Opponent == nil || !v2.Opponent.Solved || v2.Opponent.Guesses != 1 {
		t.Fatalf("opponent: %+v", v2.Opponent)
	}
	b, _ := json.Marshal(v2)
	if strings.Contains(string(b), w) {
		t.Fatalf("seat 2 view leaks %s: %s", w, b)
	}
	if (Engine{}).ToMove(s) != 0 {
		t.Fatal("duels have no turns")
	}
}

func TestDecodeRefusesCorruptState(t *testing.T) {
	good := secret(t, mustNew(t, 1, game.Options{Topic: "calm"}, 1))
	cases := map[string]func(s *state){
		"lower word":    func(s *state) { s.Word = strings.ToLower(s.Word) },
		"mode":          func(s *state) { s.Mode = "speed" },
		"topic":         func(s *state) { s.Topic = "x" },
		"guess length":  func(s *state) { s.Seats[0].Guesses = []string{"AB"} },
		"max guesses":   func(s *state) { s.MaxGuesses = 99 },
		"no seats":      func(s *state) { s.Seats = nil },
		"solved flag":   func(s *state) { s.Seats[0].Solved = true },
		"too many hint": func(s *state) { s.Seats[0].Hints = []string{"a", "b", "c", "d"} },
	}
	for name, f := range cases {
		c := *good
		c.Seats = append([]seat{}, good.Seats...)
		f(&c)
		raw, _ := json.Marshal(c)
		if _, err := (Engine{}).Apply(raw, 1, json.RawMessage(`{"hint":true}`)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestCatalog(t *testing.T) {
	b, _ := json.Marshal(Engine{}.Catalog())
	for _, want := range []string{`"topics"`, `"modes"`, `"calm"`, `"mystery"`, `"blurb"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("catalog %s lacks %s", b, want)
		}
	}
}

// FuzzMoves plays any byte stream as a sequence of moves and checks that
// nothing panics and no active view ever carries the answer
func FuzzMoves(f *testing.F) {
	f.Add(uint64(1), uint8(0), uint8(2), []byte("hgABCDEhh"))
	f.Add(uint64(42), uint8(2), uint8(1), []byte("ggggggggghhhh"))
	f.Add(uint64(7), uint8(1), uint8(2), []byte{0, 1, 2, 3, 250, 'w', 'w'})
	f.Fuzz(func(t *testing.T, seed uint64, modeIdx, players uint8, ops []byte) {
		o := game.Options{Mode: modes[int(modeIdx)%len(modes)].id}
		if seed%3 != 0 || o.Mode != "mystery" {
			o.Topic = banks[int(seed/3)%len(banks)].id
		}
		np := 1 + int(players)%2
		s, err := Engine{}.New(seed, o, np)
		if err != nil {
			t.Fatal(err)
		}
		word := secret(t, s).Word
		for k, op := range ops {
			seat := int8(1 + int(op>>7))
			var mv string
			switch op % 4 {
			case 0:
				mv = `{"hint":true}`
			case 1:
				mv = `{"guess":"` + word + `"}`
			case 2:
				// a guess built from the following bytes, often illegal on purpose
				end := min(len(ops), k+1+len(word))
				mv = fmt.Sprintf(`{"guess":%q}`, string(ops[k+1:end]))
			default:
				g := []byte(word)
				g[int(op)%len(g)] = 'A' + op%26
				mv = `{"guess":"` + string(g) + `"}`
			}
			next, err := Engine{}.Apply(s, seat, json.RawMessage(mv))
			if err != nil {
				if !errors.Is(err, game.ErrIllegal) {
					t.Fatalf("move %s: %v", mv, err)
				}
				continue
			}
			s = next
			over := Engine{}.Result(s).Over
			for v := int8(1); v <= int8(np); v++ {
				vw := view(t, s, v)
				b, _ := json.Marshal(vw)
				if !over && !vw.Solved && strings.Contains(string(b), word) {
					t.Fatalf("seat %d sees %s while active: %s", v, word, b)
				}
				if !over && (vw.Answer != nil || vw.Fact != nil || vw.Clue != nil) {
					t.Fatalf("seat %d sees the answer fields while active", v)
				}
			}
		}
	})
}

func BenchmarkNew(b *testing.B) {
	o := game.Options{Topic: "calm", Mode: "anagram"}
	for i := range b.N {
		if _, err := (Engine{}).New(uint64(i), o, 2); err != nil {
			b.Fatal(err)
		}
	}
}
