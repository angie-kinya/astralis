package agent

import (
	"strings"
	"testing"

	"astralis/gateway/internal/store"

	"github.com/stretchr/testify/require"
)

func TestShapeFor(t *testing.T) {
	cases := []struct {
		name                string
		intent              string
		words               int
		lastAsked, retrieve bool
		want                Shape
	}{
		{"a one-word greeting gets the floor", "greeting", 1, false, false, Shape{Words: 8, MayAsk: true}},
		{"a long vent hits the ceiling", "vent", 200, false, false, Shape{Words: 110, MayAsk: true}},
		{"a question last turn means none this turn", "vent", 10, true, false, Shape{Words: 24, MayAsk: false}},
		{"advice may cite when research ran", "seek_advice", 30, false, true, Shape{Words: 72, MayAsk: true, MayAdvise: true, Cite: true}},
		{"a vent never cites, even with research", "vent", 30, false, true, Shape{Words: 52, MayAsk: true}},
		{"evidence never asks", "ask_evidence", 10, false, true, Shape{Words: 90, MayAdvise: true, Cite: true}},
		{"an unknown intent falls back to reflect", "mystery", 10, false, false, Shape{Words: 35, MayAsk: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, shapeFor(c.intent, c.words, c.lastAsked, c.retrieve))
		})
	}
}

func TestShapeTokensFollowTheBudget(t *testing.T) {
	require.Equal(t, 160, Shape{Words: 8}.MaxTokens(), "a floor so a short reply is never cut mid-word")
	require.Equal(t, 504, Shape{Words: 220}.MaxTokens())
	require.Equal(t, 77, Shape{Words: 110}.scaled(0.7).Words, "terse scales the budget")
	require.Equal(t, 8, Shape{Words: 8}.scaled(0.7).Words, "but never below a usable floor")
	ins := Shape{Words: 40}.Instructions()
	require.Contains(t, ins, "Do not ask a question")
	require.Contains(t, ins, "Do not use [n] markers")
}

func TestDecideSetsShapeAndMoves(t *testing.T) {
	tired := store.Pattern{NRecent: 5, ExhRecent: 2.6, ExhBefore: 1.5}

	t.Run("the question rule is code", func(t *testing.T) {
		p := Decide(Analysis{Intent: "vent"}, true, "ugh", Situation{LastAsked: true})
		require.False(t, p.Shape.MayAsk)
	})
	t.Run("the terse voice gets a smaller budget", func(t *testing.T) {
		steady := Decide(Analysis{Intent: "reflect"}, true, strings.Repeat("word ", 60), Situation{Variant: "steady"})
		terse := Decide(Analysis{Intent: "reflect"}, true, strings.Repeat("word ", 60), Situation{Variant: "terse"})
		require.Less(t, terse.Shape.Words, steady.Shape.Words)
	})
	t.Run("a recurring demand is noticed once", func(t *testing.T) {
		sit := Situation{Pattern: store.Pattern{RecurringDemand: "manager_relationship"}}
		p := Decide(Analysis{Intent: "vent"}, true, "he did it again", sit)
		require.Equal(t, []string{"notice_demand"}, p.Moves)
		require.Contains(t, p.Guidance, "things with their manager")

		sit.Moved = map[string]bool{"notice_demand": true}
		require.Empty(t, Decide(Analysis{Intent: "vent"}, true, "he did it again", sit).Moves, "one noticing move per session")
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

func TestLint(t *testing.T) {
	sh := Shape{Words: 20, MayAsk: true}
	cases := []struct {
		name  string
		reply string
		shape Shape
		prev  []string
		want  []string
	}{
		{"a clean reply", "Same targets nobody hit is a planning problem.", sh, nil, []string{}},
		{"two questions", "Was it Friday? Or Monday?", sh, nil, []string{"question"}},
		{"a question when none was allowed", "How did it go?", Shape{Words: 20}, nil, []string{"question"}},
		{"too long", strings.Repeat("word ", 40), sh, nil, []string{"length"}},
		{"stock phrase", "It sounds like a hard week.", sh, nil, []string{"stock_phrase"}},
		{"an em dash", "Monday again—same targets.", sh, nil, []string{"dash"}},
		{"a hyphenated word is fine", "A month-end audit.", sh, nil, []string{}},
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

func TestRepairNoteNamesEachFix(t *testing.T) {
	n := repairNote([]string{"question", "dash"}, "Draft text?")
	require.Contains(t, n, "remove the question")
	require.Contains(t, n, "commas or full stops")
	require.True(t, strings.HasSuffix(n, "Draft text?"))
}

func TestPickVariant(t *testing.T) {
	names := []string{"steady", "terse"}
	t.Run("explores while any voice is under-rated", func(t *testing.T) {
		seen := map[string]bool{}
		for range 200 {
			seen[PickVariant(map[string][2]int{"steady": {500, 0}}, names, 20)] = true
		}
		require.Len(t, seen, 2, "terse has no ratings, so both must be tried")
	})
	t.Run("exploits a clearly better voice", func(t *testing.T) {
		stats := map[string][2]int{"steady": {10, 90}, "terse": {90, 10}}
		for range 100 {
			require.Equal(t, "terse", PickVariant(stats, names, 20))
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

func TestBuildSystemPutsTheShapeLast(t *testing.T) {
	p := Plan{Guidance: "Brief.", Shape: Shape{Words: 30}}
	sys := buildSystem("terse", nil, []store.Fact{{Kind: "commitment", Subject: "walk at lunch", Detail: "said she would try", Status: "open"}}, nil, p)
	require.True(t, strings.HasPrefix(sys, Character))
	require.Contains(t, sys, Variants["terse"].Line)
	require.Contains(t, sys, `commitment "walk at lunch": said she would try (open)`)
	require.Less(t, strings.Index(sys, "HOW TO APPROACH"), strings.Index(sys, "THIS REPLY"))
}
