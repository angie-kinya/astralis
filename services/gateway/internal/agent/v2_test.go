package agent

import (
	"slices"
	"strings"
	"testing"

	"astralis/gateway/internal/store"

	"github.com/stretchr/testify/require"
)

func TestShapeFor(t *testing.T) {
	cases := []struct {
		name     string
		intent   string
		words    int
		retrieve bool
		want     Shape
	}{
		{"a one-word greeting gets the floor", "greeting", 1, false, Shape{Words: 15}},
		{"a long vent hits the ceiling", "vent", 200, false, Shape{Words: 140}},
		{"a short vent mirrors the person above the floor", "vent", 10, false, Shape{Words: 35}},
		{"advice may cite when research ran", "seek_advice", 30, true, Shape{Words: 77, MayAdvise: true, Cite: true}},
		{"a vent never cites, even with research", "vent", 30, true, Shape{Words: 59}},
		{"evidence gets its floor", "ask_evidence", 10, true, Shape{Words: 90, MayAdvise: true, Cite: true}},
		{"an unknown intent falls back to reflect", "mystery", 10, false, Shape{Words: 45}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, shapeFor(c.intent, c.words, c.retrieve))
		})
	}
}

func TestBudgetsNeverDropBelowTheFloor(t *testing.T) {
	want := map[string][2]int{
		"vent": {35, 140}, "reflect": {45, 150}, "report_progress": {25, 110}, "greeting": {15, 45},
		"off_topic": {20, 50}, "seek_advice": {70, 180}, "ask_evidence": {90, 220},
	}
	require.Equal(t, want, budgets)
	for intent, b := range budgets {
		for _, n := range []int{0, 1, 5, 20, 80, 500} {
			w := shapeFor(intent, n, false).Words
			require.GreaterOrEqual(t, w, b[0], "%s at %d words", intent, n)
			require.LessOrEqual(t, w, b[1], "%s at %d words", intent, n)
		}
		require.Less(t, shapeFor(intent, 1, false).Words, shapeFor(intent, 500, false).Words, "%s still mirrors", intent)
	}
}

func TestShapeTokensFollowTheBudget(t *testing.T) {
	require.Equal(t, 160, Shape{Words: 8}.MaxTokens(), "a floor so a short reply is never cut mid-word")
	require.Equal(t, 504, Shape{Words: 220}.MaxTokens())
	ins := Shape{Words: 40}.Instructions()
	require.Contains(t, ins, "Do not ask a question", "the zero value forbids")
	require.Contains(t, ins, "Do not use [n] markers")
	require.Contains(t, Shape{Words: 40, Ask: AskAllow}.Instructions(), "A question is optional; at most one.")
	require.Contains(t, Shape{Words: 40, Ask: AskInvite}.Instructions(), "End with one easy, open question about them, if it feels natural.")
	require.NotContains(t, Shape{Words: 40, Ask: AskInvite}.Instructions(), "Do not ask")
}

func TestCadence(t *testing.T) {
	no, yes := false, true
	cases := []struct {
		name      string
		intent    string
		words     int
		userAsked bool
		listening bool
		asked     []bool
		quiet     int
		want      Ask
	}{
		{"the first reply to a short vent invites", "vent", 6, no, no, nil, 2, AskInvite},
		{"the previous reply asked, so none now", "vent", 6, no, no, []bool{no, yes}, 2, AskForbid},
		{"evidence never asks", "ask_evidence", 6, no, no, nil, 2, AskForbid},
		{"off topic never asks", "off_topic", 6, no, no, nil, 2, AskForbid},
		{"they asked us something, so we answer", "reflect", 6, yes, no, nil, 2, AskForbid},
		{"advice allows but does not invite", "seek_advice", 6, no, no, nil, 2, AskAllow},
		{"listening never invites", "vent", 6, no, yes, nil, 2, AskAllow},
		{"a question two replies back holds the invite", "vent", 6, no, no, []bool{yes, no}, 2, AskAllow},
		{"two quiet replies invite again", "reflect", 6, no, no, []bool{yes, no, no}, 2, AskInvite},
		{"curious invites after one quiet reply", "vent", 6, no, no, []bool{yes, no}, 1, AskInvite},
		{"a long message late in a session allows", "vent", 40, no, no, []bool{no, no, no}, 2, AskAllow},
		{"a long message early in a session invites", "vent", 40, no, no, []bool{no, no}, 2, AskInvite},
		{"progress and greetings invite too", "report_progress", 8, no, no, nil, 2, AskInvite},
		{"a greeting invites", "greeting", 1, no, no, nil, 2, AskInvite},
		{"no intent (analyser down) allows", "", 6, no, no, nil, 2, AskAllow},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, cadence(c.intent, c.words, c.userAsked, c.listening, c.asked, c.quiet))
		})
	}
}

func TestUserAsked(t *testing.T) {
	for msg, want := range map[string]bool{
		"how does this thing actually work":        true,
		"is it normal to dread opening email":      true,
		"are you a real person":                    true,
		"so tired?":                                true,
		"when am I supposed to do the actual work": false,
		"seven meetings today":                     false,
		"how":                                      false,
		"fine":                                     false,
		"What, honestly, is the point of these reviews": true,
	} {
		require.Equal(t, want, userAsked(msg), msg)
	}
}

func TestWantsListening(t *testing.T) {
	for msg, want := range map[string]bool{
		"I just need to vent":                         true,
		"i just really need to rant for a sec":        true,
		"I DON'T WANT ADVICE ok":                      true,
		"I don’t want advice, I want to moan":         true,
		"not looking for advice, just annoyed":        true,
		"can you just listen":                         true,
		"please don't try to fix this":                true,
		"don't fix it, it's fine":                     true,
		"let me rant for a minute":                    true,
		"I need to vent":                              true,
		"what should I do about my manager":           false,
		"my manager gave me advice I didn't want":     false,
		"I tried to fix the rota myself":              false,
		"I want to get better at listening to people": false,
	} {
		require.Equal(t, want, wantsListening(msg), msg)
	}
}

func TestDecideSetsShapeAndMoves(t *testing.T) {
	tired := store.Pattern{NRecent: 5, ExhRecent: 2.6, ExhBefore: 1.5}

	t.Run("the question rule is code", func(t *testing.T) {
		p := Decide(Analysis{Intent: "vent"}, true, "ugh", Situation{Asked: []bool{true}})
		require.Equal(t, AskForbid, p.Shape.Ask)
	})
	t.Run("a short vent after quiet replies invites a question", func(t *testing.T) {
		p := Decide(Analysis{Intent: "vent"}, true, "ugh", Situation{Asked: []bool{false, false}})
		require.Equal(t, AskInvite, p.Shape.Ask)
	})
	t.Run("the voices differ only in question cadence", func(t *testing.T) {
		sit := Situation{Asked: []bool{true, false}}
		warm := Decide(Analysis{Intent: "vent"}, true, "ugh", Situation{Asked: sit.Asked, Variant: "warm"})
		curious := Decide(Analysis{Intent: "vent"}, true, "ugh", Situation{Asked: sit.Asked, Variant: "curious"})
		require.Equal(t, AskAllow, warm.Shape.Ask)
		require.Equal(t, AskInvite, curious.Shape.Ask)
		warm.Shape.Ask = curious.Shape.Ask
		require.Equal(t, warm, curious, "nothing else changes")
	})
	t.Run("an old or unknown variant behaves like the default", func(t *testing.T) {
		sit := Situation{Asked: []bool{true, false}}
		def := Decide(Analysis{Intent: "vent"}, true, "ugh", sit)
		for _, v := range []string{"terse", "steady", "nonsense", ""} {
			sit.Variant = v
			require.Equal(t, def, Decide(Analysis{Intent: "vent"}, true, "ugh", sit), v)
		}
	})
	t.Run("a recurring demand is noticed once", func(t *testing.T) {
		sit := Situation{Pattern: store.Pattern{RecurringDemand: "manager_relationship"}}
		p := Decide(Analysis{Intent: "vent"}, true, "he did it again", sit)
		require.Equal(t, []string{"notice_demand"}, p.Moves)
		require.Contains(t, p.Guidance, "things with their manager")

		sit.Moved = map[string]bool{"notice_demand": true}
		require.Empty(t, Decide(Analysis{Intent: "vent"}, true, "he did it again", sit).Moves, "one noticing move per session")
	})
	t.Run("listen_only does not use up the session's noticing", func(t *testing.T) {
		sit := Situation{Pattern: store.Pattern{RecurringDemand: "workload"}, Moved: map[string]bool{moveListen: true}}
		require.Equal(t, []string{"notice_demand"}, Decide(Analysis{Intent: "vent"}, true, "again", sit).Moves)
	})
	t.Run("rising exhaustion is noticed when there is enough evidence", func(t *testing.T) {
		require.Equal(t, []string{"notice_exhaustion"}, Decide(Analysis{Intent: "reflect"}, true, "so tired", Situation{Pattern: tired}).Moves)
		thin := tired
		thin.NRecent = 2
		require.Empty(t, Decide(Analysis{Intent: "reflect"}, true, "so tired", Situation{Pattern: thin}).Moves)
	})
	t.Run("never on a greeting or a request for advice", func(t *testing.T) {
		require.Empty(t, Decide(Analysis{Intent: "greeting"}, true, "hi", Situation{Pattern: tired}).Moves)
		require.Empty(t, Decide(Analysis{Intent: "seek_advice"}, true, "what do I do", Situation{Pattern: tired}).Moves)
	})
}

func TestDecideListenOnly(t *testing.T) {
	listening := map[string]bool{moveListen: true}
	cases := []struct {
		name       string
		an         Analysis
		analysed   bool
		msg        string
		moved      map[string]bool
		wantMove   bool // listen_only recorded this turn
		wantListen bool // advice off and no invite this turn
	}{
		{"a phrase starts listen-only", Analysis{Intent: "vent"}, true, "I just need to vent", nil, true, true},
		{"the analyser's no starts it", Analysis{Intent: "vent", WantsAdvice: "no"}, true, "ugh, today", nil, true, true},
		{"unclear does not", Analysis{Intent: "vent", WantsAdvice: "unclear"}, true, "ugh, today", nil, false, false},
		{"it holds for the session without being recorded twice", Analysis{Intent: "vent"}, true, "ugh, today", listening, false, true},
		{"it holds on reflect too", Analysis{Intent: "reflect"}, true, "maybe it's me", listening, false, true},
		{"asking for advice lifts it for that turn", Analysis{Intent: "seek_advice", WantsAdvice: "yes"}, true, "ok what should I do", listening, false, false},
		{"a phrase inside an advice request is not recorded", Analysis{Intent: "seek_advice"}, true, "I don't want advice, well, maybe", nil, false, false},
		{"the phrase check works with the analyser down", Analysis{}, false, "please just listen", nil, true, true},
		{"and an earlier move still holds with it down", Analysis{}, false, "ugh", listening, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := Decide(c.an, c.analysed, c.msg, Situation{Moved: c.moved})
			require.Equal(t, c.wantMove, slices.Contains(p.Moves, moveListen))
			if c.wantListen {
				require.False(t, p.Shape.MayAdvise)
				require.NotEqual(t, AskInvite, p.Shape.Ask)
				require.Contains(t, p.Guidance, listenGuidance)
			} else {
				require.NotContains(t, p.Guidance, listenGuidance)
			}
		})
	}
	t.Run("advice intent keeps advice on when lifted", func(t *testing.T) {
		p := Decide(Analysis{Intent: "seek_advice"}, true, "ok what should I do", Situation{Moved: listening})
		require.True(t, p.Shape.MayAdvise)
	})
	t.Run("a listening vent with quiet replies allows rather than invites", func(t *testing.T) {
		p := Decide(Analysis{Intent: "vent"}, true, "ugh", Situation{Moved: listening, Asked: []bool{false, false}})
		require.Equal(t, AskAllow, p.Shape.Ask)
	})
}

func TestResolveVariant(t *testing.T) {
	require.Equal(t, "warm", DefaultVariant)
	require.Equal(t, "curious", resolveVariant("curious"))
	require.Equal(t, "warm", resolveVariant("warm"))
	for _, old := range []string{"terse", "steady", "", "WARM"} {
		require.Equal(t, DefaultVariant, resolveVariant(old), old)
	}
	require.Equal(t, 2, Variants["warm"].quiet())
	require.Equal(t, 1, Variants["curious"].quiet())
	require.Equal(t, 2, Variant{}.quiet(), "the zero value is the warm cadence")
}

func TestLint(t *testing.T) {
	sh := Shape{Words: 20, Ask: AskAllow}
	cases := []struct {
		name  string
		reply string
		shape Shape
		prev  []string
		want  []string
	}{
		{"a clean reply", "Same targets nobody hit again. That's rough.", sh, nil, []string{}},
		{"two questions", "Was it Friday? Or Monday?", sh, nil, []string{"question"}},
		{"a question when none was allowed", "How did it go?", Shape{Words: 20}, nil, []string{"question"}},
		{"too long", strings.Repeat("word ", 40), sh, nil, []string{"length"}},
		{"stock phrase", "It sounds like a hard week.", sh, nil, []string{"stock_phrase"}},
		{"an em dash", "Monday again—same targets.", sh, nil, []string{"dash"}},
		{"a hyphenated word is fine", "A month-end audit.", sh, nil, []string{}},
		{"jargon", "That's a planning problem dressed up as a motivation problem.", sh, nil, []string{"plain_words"}},
		{"repeated opener", "That is a lot of meetings.", sh, []string{"That is a long day."}, []string{"repeated_opener"}},
		{"an opener from long ago is fine", "That is a lot.", sh,
			[]string{"That is old.", "a", "b", "c", "d", "e"}, []string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, Lint(c.reply, c.shape, c.prev))
		})
	}
}

func TestLintQuestionFollowsAsk(t *testing.T) {
	none, one, two := "Seven meetings is a lot.", "Seven meetings is a lot. Catching up tonight?", "Seven? Tonight too?"
	cases := []struct {
		ask   Ask
		reply string
		flag  bool
	}{
		{AskInvite, none, false}, // we invite, we never force
		{AskInvite, one, false},
		{AskInvite, two, true},
		{AskAllow, none, false},
		{AskAllow, one, false},
		{AskAllow, two, true},
		{AskForbid, none, false},
		{AskForbid, one, true},
		{AskForbid, two, true},
	}
	for _, c := range cases {
		t.Run(c.ask.String()+"/"+c.reply, func(t *testing.T) {
			got := slices.Contains(Lint(c.reply, Shape{Words: 40, Ask: c.ask}, nil), "question")
			require.Equal(t, c.flag, got)
		})
	}
}

func TestPlainWords(t *testing.T) {
	for _, c := range []struct {
		reply string
		plain bool
	}{
		{"Oof, seven. That's the whole day gone. Are you catching up tonight?", true},
		{"Everything okay at home?", true}, // one long word is never enough
		{"Very normal, especially when email is where new work lands without warning.", true},
		{"Organisational unpredictability and interpersonal conflict are fundamentally exhausting.", false},
		{"That sounds like a mismatch between you and the role.", false},
		{"Working like that is unsustainable.", false},
		{"You could leverage your manager's support.", false},
		{"Try to optimise your mornings.", false},
		{"A simple framework helps.", false},
		{"It might help to set boundaries with him.", false},
		{"There are no boundaries in that office, people just walk in.", true}, // the plain sense is fine
		{"That's cognitive load piling up.", false},
	} {
		require.Equal(t, c.plain, plainWords(c.reply), c.reply)
	}
}

func TestSyllables(t *testing.T) {
	for w, n := range map[string]int{
		"day": 1, "make": 1, "table": 2, "tired": 2, "meeting": 2, "everything": 4,
		"organisational": 6, "the": 1, "a": 1, "rhythm": 1,
	} {
		require.Equal(t, n, syllables(w), w)
	}
}

func TestRepairNoteNamesEachFix(t *testing.T) {
	n := repairNote([]string{"question", "dash"}, "Draft text?")
	require.Contains(t, n, "remove the question")
	require.Contains(t, n, "commas or full stops")
	require.True(t, strings.HasSuffix(n, "Draft text?"))
}

func TestPickVariant(t *testing.T) {
	names := []string{"warm", "curious"}
	t.Run("explores while any voice is under-rated", func(t *testing.T) {
		seen := map[string]bool{}
		for range 200 {
			seen[PickVariant(map[string][2]int{"warm": {500, 0}}, names, 20)] = true
		}
		require.Len(t, seen, 2, "curious has no ratings, so both must be tried")
	})
	t.Run("exploits a clearly better voice", func(t *testing.T) {
		stats := map[string][2]int{"warm": {10, 90}, "curious": {90, 10}}
		for range 100 {
			require.Equal(t, "curious", PickVariant(stats, names, 20))
		}
	})
	t.Run("no names means the base voice", func(t *testing.T) {
		require.Equal(t, DefaultVariant, PickVariant(nil, nil, 20))
	})
}

func TestBetaSamplerMean(t *testing.T) {
	for _, ab := range [][2]float64{{3, 7}, {1, 1}, {0.5, 0.5}, {40, 10}} {
		const n = 40000
		sum := 0.0
		for range n {
			d := betaDraw(ab[0], ab[1])
			require.True(t, d >= 0 && d <= 1)
			sum += d
		}
		want := ab[0] / (ab[0] + ab[1])
		require.InDelta(t, want, sum/n, 0.01, "Beta(%v,%v)", ab[0], ab[1])
	}
}

func TestParseFactsDistrustsTheModel(t *testing.T) {
	fs, err := parseFacts(`{"facts":[
		{"kind":"person","subject":" branch manager ","detail":"moved the audit to Friday","status":"done"},
		{"kind":"person","subject":"Branch Manager","detail":"duplicate","status":"open"},
		{"kind":"diagnosis","subject":"x","detail":"y","status":"open"},
		{"kind":"commitment","subject":"leave by six","detail":"will try Thursdays","status":"done"},
		{"kind":"event","subject":"","detail":"no subject","status":"open"}]}`)
	require.NoError(t, err)
	require.Len(t, fs, 2)
	require.Equal(t, "branch manager", fs[0].Subject)
	require.Equal(t, "open", fs[0].Status, "only commitments carry a status")
	require.Equal(t, "done", fs[1].Status)
	_, err = parseFacts("nope")
	require.Error(t, err)
}

// the anchors are what the model imitates most, so each one must pass the
// lint it will be held to, with a question only where one is allowed
func TestAnchorsPassTheLint(t *testing.T) {
	n := 0
	for _, line := range strings.Split(Character, "\n") {
		if reply, ok := strings.CutPrefix(line, "You: "); ok {
			n++
			require.Empty(t, Lint(strings.Trim(reply, `"`), Shape{Words: 60, Ask: AskAllow}, nil), reply)
		}
	}
	require.Equal(t, 6, n)
	require.Contains(t, Character, "HONESTY (fixed)\nYou are an AI.")
}

func TestBuildSystemPutsTheShapeLast(t *testing.T) {
	p := Plan{Guidance: "Brief.", Shape: Shape{Words: 30}}
	sys := buildSystem("curious", nil, []store.Fact{{Kind: "commitment", Subject: "walk at lunch", Detail: "said she would try", Status: "open"}}, nil, p)
	require.True(t, strings.HasPrefix(sys, Character))
	require.Contains(t, sys, Variants["curious"].Line)
	require.Contains(t, sys, `commitment "walk at lunch": said she would try (open)`)
	require.Less(t, strings.Index(sys, "HOW TO APPROACH"), strings.Index(sys, "THIS REPLY"))
}
