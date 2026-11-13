package agent

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"astralis/gateway/internal/llm"
	"astralis/gateway/internal/store"

	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------- never truncate

func TestReplyCeilingIsDecoupledFromLength(t *testing.T) {
	for effort, want := range map[string]int{"": 1024, "low": 2048, "medium": 4096, "high": 4096} {
		require.Equal(t, want, replyCeiling(effort), effort)
	}
}

func TestComplete(t *testing.T) {
	for reply, want := range map[string]bool{
		"That's a long week.":                  true,
		"Catching up tonight?":                 true,
		"Go for it!":                           true,
		"Fine can cover a lot…":                true,
		`She said "not again."`:                true,
		"(which is fair.)":                     true,
		"Sleep matters [2].":                   true,
		"Sleep matters [1, 3]":                 false, // the marker hides a missing stop
		"Good for you 🎉":                       true,
		"Good for you **well done.**":          true,
		"That's the whole day gone before":     false,
		"Sounds":                               false,
		"Seven meetings, and":                  false,
		"Two things you could try:":            false,
		"":                                     false,
		"   That's a long week.   \n":          true,
		"It lands at 3.5":                      false,
		"Check email at set times, e.g":        false,
		"Ok.\n\nOne more thing, the bit about": false,
	} {
		require.Equal(t, want, complete(reply), "%q", reply)
	}
}

func TestTrimToSentence(t *testing.T) {
	cases := []struct{ in, want string }{
		{"That's a lot. Seven meetings before", "That's a lot."},
		{"Was it Friday? Or was it the week before", "Was it Friday?"},
		{"Oof. That's rough! And then the", "Oof. That's rough!"},
		{`She said "not again." And then`, `She said "not again."`},
		{"Sleep helps [1]. Support helps [2, 3]. And the", "Sleep helps [1]. Support helps [2, 3]."},
		{"Sleep helps [1] and the", ""},
		{"The pay rose 3.5 percent and then", ""}, // a decimal point is not an end
		{"no sentence ever finished here", ""},
		{"One.\n\nTwo is the bit", "One."},
		{"All complete.", "All complete."},
	}
	for _, c := range cases {
		require.Equal(t, c.want, trimToSentence(c.in), "%q", c.in)
	}
}

func TestFinish(t *testing.T) {
	cases := []struct {
		name string
		in   string
		cut  bool
		want string
		ok   bool
	}{
		{"a complete reply passes untouched", "That's a long week.", false, "That's a long week.", true},
		{"a cut reply loses its half sentence", "That's a long week. Before you", true, "That's a long week.", true},
		{"a cut reply that stopped on a stop keeps it", "That's a long week.", true, "That's a long week.", true},
		{"a cut reply with no full sentence fails", "That's the whole day gone before", true, "", false},
		{"an uncut reply missing its stop gets one", "Go for it, I'm listening", false, "Go for it, I'm listening.", true},
		{"a dangling comma becomes the stop", "Go for it,", false, "Go for it.", true},
		{"an empty reply fails", "  ", false, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := finish(c.in, c.cut)
			require.Equal(t, c.ok, ok)
			require.Equal(t, c.want, got)
			if ok {
				require.True(t, complete(got), "whatever ships is complete")
			}
		})
	}
}

// scripted is a Model that replays responses in order and keeps every request
type scripted struct {
	out  []llm.Response
	errs []error
	reqs []llm.Request
}

func (s *scripted) Generate(_ context.Context, r llm.Request) (llm.Response, error) {
	s.reqs = append(s.reqs, r)
	i := len(s.reqs) - 1
	var err error
	if i < len(s.errs) {
		err = s.errs[i]
	}
	return s.out[min(i, len(s.out)-1)], err
}

func TestRespondRetriesOnceWithDoubleTheCeiling(t *testing.T) {
	full := llm.Response{Text: "That's a long week. Are you catching up tonight?", StopReason: "end_turn", Usage: llm.Usage{OutputTokens: 12}}
	cutOff := llm.Response{Text: "That's a long week. Are you", StopReason: stopMaxTokens, Usage: llm.Usage{OutputTokens: 1024}}

	cases := []struct {
		name     string
		effort   string
		out      []llm.Response
		errs     []error
		limits   []int
		wantText string
		wantCut  bool
	}{
		{"a reply that ends is called once at the ceiling", "", []llm.Response{full}, nil, []int{1024}, full.Text, false},
		{"a cut reply is retried with twice the room", "", []llm.Response{cutOff, full}, nil, []int{1024, 2048}, full.Text, false},
		{"effort raises the ceiling", "high", []llm.Response{cutOff, full}, nil, []int{4096, 8192}, full.Text, false},
		{"cut twice stays cut, for finish to trim", "", []llm.Response{cutOff, cutOff}, nil, []int{1024, 2048}, cutOff.Text, true},
		{"a failed retry leaves the cut reply standing", "", []llm.Response{cutOff, {}}, []error{nil, errors.New("overloaded")},
			[]int{1024, 2048}, cutOff.Text, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := &scripted{out: c.out, errs: c.errs}
			a := &Agent{model: m, cfg: Config{RespondEffort: c.effort}}
			run := store.Run{StagesMS: map[string]int64{}}
			d, err := a.respond(context.Background(), "sys", nil, &run)
			require.NoError(t, err)
			require.Equal(t, c.wantText, d.text)
			require.Equal(t, c.wantCut, d.cut)
			var limits []int
			for _, r := range m.reqs {
				limits = append(limits, r.MaxTokens)
				require.Equal(t, c.effort, r.Effort)
			}
			require.Equal(t, c.limits, limits)
			if len(m.reqs) > 1 {
				require.Contains(t, run.StagesMS, "retry", "the retry is visible in the trace")
			}
		})
	}
	t.Run("usage from both calls is counted", func(t *testing.T) {
		run := store.Run{}
		a := &Agent{model: &scripted{out: []llm.Response{cutOff, full}}}
		_, err := a.respond(context.Background(), "sys", nil, &run)
		require.NoError(t, err)
		require.Equal(t, 1036, run.OutputTokens)
		require.Equal(t, "end_turn", run.StopReason)
	})
	t.Run("an empty reply is an error", func(t *testing.T) {
		a := &Agent{model: &scripted{out: []llm.Response{{Text: " ", StopReason: "end_turn"}}}}
		_, err := a.respond(context.Background(), "sys", nil, &store.Run{})
		require.Error(t, err)
	})
}

func TestLintDraftFlagsACutEvenOnAFullStop(t *testing.T) {
	sh := Shape{Band: Band{40, 120}, Ask: AskAllow}
	require.Equal(t, []string{"truncated"}, lintDraft(draft{text: "That's a long week.", cut: true}, sh, nil))
	require.Equal(t, []string{"truncated"}, lintDraft(draft{text: "That's a long week before", cut: true}, sh, nil), "never twice")
	require.Empty(t, lintDraft(draft{text: "That's a long week."}, sh, nil))
}

// ---------------------------------------------------------------- the stance table

func TestChooseStance(t *testing.T) {
	cases := []struct {
		name string
		s    turnState
		want Stance
	}{
		{"a greeting greets", turnState{intent: "greeting", asked: true}, StanceGreet},
		{"off topic stays off topic", turnState{intent: "off_topic"}, StanceOffTopic},
		{"asking for evidence informs", turnState{intent: "ask_evidence"}, StanceInform},
		{"the analyser's inform hint informs", turnState{intent: "reflect", hint: "inform"}, StanceInform},
		{"a research question in words informs", turnState{intent: "vent", evidence: true}, StanceInform},
		{"evidence beats listening: they asked", turnState{intent: "vent", evidence: true, listening: true}, StanceInform},
		{"a strong ask advises on turn one", turnState{intent: "seek_advice", asked: true, topicTurns: 1}, StanceAdvise},
		{"a strong ask lifts listen-only", turnState{intent: "seek_advice", asked: true, listening: true}, StanceAdvise},
		{"listen-only holds", turnState{intent: "reflect", listening: true, topicTurns: 4}, StanceListen},
		{"listen-only beats an implicit ask", turnState{intent: "reflect", softAsked: true, listening: true, topicTurns: 2}, StanceListen},
		{"an implicit ask on turn one explores", turnState{intent: "reflect", softAsked: true, topicTurns: 1}, StanceExplore},
		{"an implicit ask on turn two advises", turnState{intent: "reflect", softAsked: true, topicTurns: 2}, StanceAdvise},
		{"an implicit ask beats venting on turn two", turnState{intent: "vent", softAsked: true, topicTurns: 2}, StanceAdvise},
		{"a vent listens", turnState{intent: "vent", topicTurns: 1}, StanceListen},
		{"a vent keeps listening on turn three", turnState{intent: "vent", topicTurns: 3}, StanceListen},
		{"the listen hint listens", turnState{intent: "reflect", hint: "listen"}, StanceListen},
		{"a heavy message listens", turnState{intent: "reflect", depth: "heavy"}, StanceListen},
		{"a third turn on a topic advises", turnState{intent: "reflect", topicTurns: 3}, StanceAdvise},
		{"a second turn explores", turnState{intent: "reflect", topicTurns: 2}, StanceExplore},
		{"progress explores", turnState{intent: "report_progress", topicTurns: 1}, StanceExplore},
		{"no analysis explores", turnState{}, StanceExplore},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, chooseStance(c.s))
		})
	}
}

func TestAskPhrases(t *testing.T) {
	cases := []struct {
		msg                    string
		strong, soft, evidence bool
	}{
		{"i need help with my manager", true, false, false},
		{"I need some advice", true, false, false},
		{"how do i stop checking slack at night", true, false, false},
		{"what should i do", true, false, false},
		{"what would you actually do in my shoes", true, false, false},
		{"any tips for mornings?", true, false, false},
		{"I don't even know where to start", false, true, false},
		{"where do I even start", true, false, false},
		{"i'm having trouble lately finding motivation at work", false, true, false},
		{"struggling to switch off", false, true, false},
		{"I can't seem to get going", false, true, false},
		{"what does the research say about four day weeks", false, false, true},
		{"is there any evidence that it works", false, false, true},
		{"studies show what exactly?", false, false, true},
		{"sources?", false, false, true},
		{"seven meetings today", false, false, false},
		{"I study at night for my exams", false, false, false},
		{"I work in computer science research", false, false, false},
		{"my manager helped me out today", false, false, false},
	}
	for _, c := range cases {
		require.Equal(t, c.strong, strongAsk.MatchString(c.msg), "strong %q", c.msg)
		require.Equal(t, c.soft, softAsk.MatchString(c.msg), "soft %q", c.msg)
		require.Equal(t, c.evidence, evidenceAsk.MatchString(c.msg), "evidence %q", c.msg)
	}
}

func TestOfferReplies(t *testing.T) {
	offer := "That sounds like a slow start to every day. Want a couple of ideas, or would you rather keep talking it through?"
	cases := []struct {
		last, msg          string
		accepted, declined bool
	}{
		{offer, "yes please", true, false},
		{offer, "sure, go on", true, false},
		{offer, "ideas would be good", true, false},
		{offer, "no, I'd rather keep talking", false, true},
		{offer, "nah", false, true},
		{offer, "let's keep talking", false, true},
		{offer, "no idea what to do honestly", false, false}, // an ask, not a no
		{offer, "no, some ideas would help", false, false},
		{offer, "no, it started when the new lead arrived and nothing has felt right since", false, false},
		{offer, "yes, and also my manager has been on at me about deadlines all week and honestly I'm done", false, false},
		{"That sounds like a slow start.", "yes", false, false}, // nothing was offered
	}
	for _, c := range cases {
		require.Equal(t, c.accepted, acceptedOffer(c.last, c.msg), "accepted %q", c.msg)
		require.Equal(t, c.declined, offerMade.MatchString(c.last) && declinedOffer(c.msg), "declined %q", c.msg)
	}
}

// the screenshot conversation, and the other shapes a conversation takes,
// end to end through Decide
func TestDecideStances(t *testing.T) {
	motivation1 := "i'm having trouble lately finding motivation at work to complete my tasks"
	motivation2 := "it's more of nothing feels worth starting once i get to my sit to begin the day. i love the work that i do always, " +
		"development.. tech..engineering has always been my passion but lately finding the motivation to do so at work has been low"
	offer := "That's a slow way to start every day. Want a couple of ideas, or would you rather keep talking it through?"
	listening := map[string]bool{moveListen: true}

	cases := []struct {
		name       string
		an         Analysis
		msg        string
		sit        Situation
		stance     Stance
		advice     Advice
		turns      int
		retrieve   bool
		cite       CiteMode
		listenMove bool
	}{
		{"motivation, turn one: a problem named, so explore with research in words",
			Analysis{Intent: "reflect", StanceHint: "advise", Topic: "motivation to complete work tasks"}, motivation1,
			Situation{}, StanceExplore, AdviceNone, 1, true, CiteWords, false},
		{"motivation, turn two: same thread, asked before, so advise from research",
			Analysis{Intent: "vent", StanceHint: "explore", Topic: "lost motivation for work they love"}, motivation2,
			Situation{Topic: store.TopicState{Topic: "motivation to complete work tasks", Turns: 1}, Earlier: []string{motivation1}, Asked: []bool{true}},
			StanceAdvise, AdviceGive, 2, true, CiteWords, false},
		{"i need help with X advises on turn one",
			Analysis{Intent: "seek_advice", Topic: "saying no to extra projects workload"}, "i need help saying no to extra projects",
			Situation{}, StanceAdvise, AdviceGive, 1, true, CiteWords, false},
		{"a rant listens, with no research",
			Analysis{Intent: "vent", Topic: "manager took credit for report", Demands: []string{"manager_relationship"}},
			"my manager took credit for my report in the all-hands, again", Situation{}, StanceListen, AdviceNone, 1, false, CiteNone, false},
		{"an evidence ask informs with markers",
			Analysis{Intent: "ask_evidence", Topic: "four day week and burnout"}, "what does the research say about four day weeks",
			Situation{}, StanceInform, AdviceAllow, 1, true, CiteMarkers, false},
		{"an evidence ask while advising still earns markers",
			Analysis{Intent: "seek_advice", Topic: "switching off in the evening"}, "how do i switch off? is there any research on it",
			Situation{}, StanceInform, AdviceAllow, 1, true, CiteMarkers, false},
		{"exploring a second turn offers ideas",
			Analysis{Intent: "reflect", Topic: "meaning in the job"}, "I used to care about the mission, now it's just tickets",
			Situation{Topic: store.TopicState{Topic: "sense of purpose and meaning", Turns: 1}, Asked: []bool{true}},
			StanceExplore, AdviceOffer, 2, true, CiteWords, false},
		{"an accepted offer advises",
			Analysis{Intent: "reflect"}, "yes please",
			Situation{Topic: store.TopicState{Topic: "meaning in the job", Turns: 2}, LastReply: offer},
			StanceAdvise, AdviceGive, 3, true, CiteWords, false},
		{"a declined offer listens and holds for the session",
			Analysis{Intent: "reflect"}, "no, I'd rather keep talking",
			Situation{Topic: store.TopicState{Topic: "meaning in the job", Turns: 2}, LastReply: offer},
			StanceListen, AdviceNone, 3, false, CiteNone, true},
		{"a third turn of exploring advises",
			Analysis{Intent: "reflect", Topic: "meaning in the job"}, "I keep wondering if it's the job or me",
			Situation{Topic: store.TopicState{Topic: "meaning in the job", Turns: 2}},
			StanceAdvise, AdviceGive, 3, true, CiteWords, false},
		{"a new topic starts exploring again",
			Analysis{Intent: "reflect", Topic: "commute length"}, "also the commute is two hours now",
			Situation{Topic: store.TopicState{Topic: "meaning in the job", Turns: 2}},
			StanceExplore, AdviceNone, 1, true, CiteWords, false},
		{"listen-only overrides an implicit ask",
			Analysis{Intent: "reflect", Topic: "motivation at work"}, "I'm struggling to get anything started",
			Situation{Moved: listening, Topic: store.TopicState{Topic: "motivation", Turns: 1}},
			StanceListen, AdviceNone, 2, false, CiteNone, false},
		{"a heavy message listens",
			Analysis{Intent: "reflect", Depth: "heavy", Topic: "exhaustion after shifts"}, "cried in the car park after my shift",
			Situation{}, StanceListen, AdviceNone, 1, false, CiteNone, false},
		{"a greeting keeps the thread without counting a turn",
			Analysis{Intent: "greeting"}, "thanks",
			Situation{Topic: store.TopicState{Topic: "meaning in the job", Turns: 2}},
			StanceGreet, AdviceNone, 2, false, CiteNone, false},
		{"a topic the library does not cover explores without a search",
			Analysis{Intent: "reflect", Topic: "office plants"}, "we got new plants in the office and it's nicer",
			Situation{}, StanceExplore, AdviceNone, 1, false, CiteNone, false},
		{"a named demand is reason enough to search",
			Analysis{Intent: "reflect", Topic: "the new rota", Demands: []string{"time_pressure"}}, "the new rota means back to back shifts",
			Situation{}, StanceExplore, AdviceNone, 1, true, CiteWords, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := Decide(c.an, true, c.msg, c.sit)
			require.Equal(t, c.stance, p.Shape.Stance)
			require.Equal(t, c.advice, p.Shape.Advice)
			require.Equal(t, c.turns, p.Topic.Turns)
			require.Equal(t, c.retrieve, p.Retrieve)
			require.Equal(t, c.cite, p.Shape.Cite)
			require.Equal(t, c.listenMove, slices.Contains(p.Moves, moveListen))
			require.Equal(t, mirror(bands[c.stance], len(strings.Fields(c.msg))), p.Shape.Band)
			if p.Retrieve && p.Topic.Topic != "" {
				require.Equal(t, p.Topic.Topic, p.Query, "the topic is the search query")
			}
		})
	}
	t.Run("a long thoughtful message gets a longer reply than a short one", func(t *testing.T) {
		an := Analysis{Intent: "reflect", Topic: "meaning in the job"}
		short := Decide(an, true, "it's the job, I think", Situation{})
		long := Decide(an, true, strings.Repeat("I keep turning it over and over in my head. ", 12), Situation{})
		require.Equal(t, StanceExplore, long.Shape.Stance)
		require.Equal(t, bands[StanceExplore], long.Shape.Band)
		require.Less(t, short.Shape.Band.Max, long.Shape.Band.Max)
	})
	t.Run("the offer is the one question, even after a reply that asked", func(t *testing.T) {
		p := Decide(Analysis{Intent: "reflect", Topic: "meaning in the job"}, true, "it just feels flat",
			Situation{Topic: store.TopicState{Topic: "meaning in the job", Turns: 1}, Asked: []bool{true}})
		require.Equal(t, AdviceOffer, p.Shape.Advice)
		require.Equal(t, AskInvite, p.Shape.Ask)
	})
	t.Run("an offer never answers a question with a question", func(t *testing.T) {
		p := Decide(Analysis{Intent: "reflect", Topic: "meaning in the job"}, true, "why does it feel so flat?",
			Situation{Topic: store.TopicState{Topic: "meaning in the job", Turns: 1}})
		require.Equal(t, AdviceNone, p.Shape.Advice)
		require.Equal(t, AskForbid, p.Shape.Ask)
	})
	t.Run("advice stays an invitation, not an interview", func(t *testing.T) {
		p := Decide(Analysis{Intent: "seek_advice", Topic: "workload"}, true, "what should I do", Situation{})
		require.Equal(t, AskForbid, p.Shape.Ask, "they asked us, so we answer")
		p = Decide(Analysis{Intent: "seek_advice", Topic: "workload"}, true, "I need help with my workload", Situation{})
		require.Equal(t, AskAllow, p.Shape.Ask)
	})
	t.Run("an implicit ask about another topic does not carry over", func(t *testing.T) {
		p := Decide(Analysis{Intent: "reflect", Topic: "commute length"}, true, "the commute is two hours now",
			Situation{Topic: store.TopicState{Topic: "motivation", Turns: 1}, Earlier: []string{"I'm struggling to get started"}})
		require.Equal(t, StanceExplore, p.Shape.Stance)
	})
}

// ---------------------------------------------------------------- topics

func TestConstruct(t *testing.T) {
	for topic, want := range map[string]string{
		"motivation to complete work tasks":   "motivation",
		"nothing feels worth starting":        "motivation",
		"lost passion for engineering":        "meaning",
		"exhausted after night shifts":        "exhaustion",
		"switching off in the evening":        "detachment",
		"sleep and rest days":                 "recovery",
		"too many meetings":                   "workload",
		"manager micromanaging":               "autonomy",
		"manager took credit for report":      "recognition",
		"conflict with a colleague":           "support",
		"redundancy rumours after the merger": "insecurity",
		"office plants":                       "",
		"":                                    "",
	} {
		require.Equal(t, want, construct(topic), topic)
	}
}

func TestSameTopic(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"motivation to complete work tasks", "lost motivation for work they love", true},
		{"motivation at work", "nothing feels worth starting", true}, // one construct
		{"meaning in the job", "meaning in the job", true},
		{"the quarterly planning offsite", "planning offsite agenda", true}, // shared words, no construct
		{"meaning in the job", "commute length", false},
		{"office plants", "quarterly planning offsite", false},
		{"", "motivation", false},
		{"motivation", "", false},
	} {
		require.Equal(t, c.want, sameTopic(c.a, c.b), "%q vs %q", c.a, c.b)
	}
}

func TestFollowTopic(t *testing.T) {
	on := store.TopicState{Topic: "motivation at work", Turns: 2}
	cases := []struct {
		name  string
		prev  store.TopicState
		topic string
		want  store.TopicState
	}{
		{"a first topic starts at one", store.TopicState{}, "motivation at work", store.TopicState{Topic: "motivation at work", Turns: 1}},
		{"the same thread counts on, under its newest name", on, "procrastinating on tasks", store.TopicState{Topic: "procrastinating on tasks", Turns: 3}},
		{"a message with no topic continues the thread", on, "", store.TopicState{Topic: "motivation at work", Turns: 3}},
		{"a new topic restarts the count", on, "commute length", store.TopicState{Topic: "commute length", Turns: 1}},
		{"no topic and no thread is nothing", store.TopicState{}, "", store.TopicState{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, followTopic(c.prev, c.topic))
		})
	}
}

func TestAskedBefore(t *testing.T) {
	earlier := []string{"I'm struggling to get started", "it's every morning"}
	require.True(t, askedBefore(earlier, 2))
	require.False(t, askedBefore(earlier, 1), "the ask was before this topic began")
	require.False(t, askedBefore(earlier, 0))
	require.False(t, askedBefore(earlier, -1))
	require.True(t, askedBefore(earlier, 9), "a count past the window reads the whole window")
	require.False(t, askedBefore(nil, 3))
}

// ---------------------------------------------------------------- research

func TestResearchUsed(t *testing.T) {
	chunks := []store.Chunk{{Content: "Psychological detachment from work during evenings predicts lower exhaustion and better sleep quality the following morning."}}
	cases := []struct {
		name   string
		reply  string
		chunks []store.Chunk
		said   string
		want   bool
	}{
		{"a plain mention of research counts", "There's decent research that a set stop time helps.", chunks, "", true},
		{"distinctive excerpt words count", "Evenings away from work tend to mean better sleep quality and less exhaustion by morning.", chunks, "", true},
		{"their own words do not count", "Evenings, sleep quality, exhaustion: that's what you said.", chunks, "my evenings wreck my sleep quality and exhaustion", false},
		{"a reply that ignores the excerpts", "Oof, seven meetings. That's the whole day gone.", chunks, "", false},
		{"no excerpts, nothing used", "There's decent research on it.", nil, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, researchUsed(c.reply, c.chunks, c.said))
		})
	}
}

func TestStrayMarkersGoOutsideMarkersMode(t *testing.T) {
	// CiteWords hands renumberCitations zero excerpts, so any marker the model
	// wrote anyway is stripped and the studies chip stays empty
	text, used := renumberCitations("There's decent research that a set stop time helps [1].", 0)
	require.Equal(t, "There's decent research that a set stop time helps.", text)
	require.Empty(t, used)
}

// ---------------------------------------------------------------- memory helpers

func TestEarlierUserDropsTheCurrentTurn(t *testing.T) {
	h := []store.Message{{Role: "user", Content: "one"}, {Role: "assistant", Content: "a"}, {Role: "user", Content: "two"}}
	require.Equal(t, []string{"one"}, earlierUser(h, "two"))
	require.Equal(t, []string{"one", "two"}, earlierUser(h, "three"), "only a stored copy of this turn is dropped")
	require.Empty(t, earlierUser(nil, "x"))
}

func TestMergeFacts(t *testing.T) {
	f := func(ids ...string) []store.Fact {
		out := []store.Fact{}
		for _, id := range ids {
			out = append(out, store.Fact{ID: id})
		}
		return out
	}
	ids := func(fs []store.Fact) []string {
		out := []string{}
		for _, x := range fs {
			out = append(out, x.ID)
		}
		return out
	}
	require.Equal(t, []string{"a", "b", "c"}, ids(mergeFacts(f("a", "b"), f("b", "c"), 6)))
	require.Equal(t, []string{"a", "b"}, ids(mergeFacts(f("a", "b"), f("c"), 2)), "never past the cap")
	require.Equal(t, []string{"c"}, ids(mergeFacts(nil, f("c", "c"), 6)), "once each")
}
