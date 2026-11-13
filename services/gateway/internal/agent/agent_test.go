package agent

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecideFallsBackToPlainRetrievalWhenAnalysisFails(t *testing.T) {
	p := Decide(Analysis{Intent: "vent", Topic: "ignored"}, false, "my manager keeps moving deadlines", Situation{})
	require.True(t, p.Retrieve)
	require.Equal(t, "my manager keeps moving deadlines", p.Query)
	require.Equal(t, StanceExplore, p.Shape.Stance, "an analysis that failed is never read")
	require.Equal(t, CiteWords, p.Shape.Cite, "nothing asked for sources")

	p = Decide(Analysis{}, false, "what does the research say about four day weeks", Situation{})
	require.Equal(t, StanceInform, p.Shape.Stance)
	require.Equal(t, CiteMarkers, p.Shape.Cite)
}

func TestDecideRoutesOnIntent(t *testing.T) {
	t.Run("greeting never retrieves", func(t *testing.T) {
		p := Decide(Analysis{Intent: "greeting", NeedsResearch: true}, true, "hello there", Situation{})
		require.False(t, p.Retrieve)
		require.NotEmpty(t, p.Guidance)
	})
	t.Run("asking for evidence always retrieves, with the topic as the query", func(t *testing.T) {
		p := Decide(Analysis{Intent: "ask_evidence", Topic: "detachment from work and sleep"}, true, "does that help?", Situation{})
		require.True(t, p.Retrieve)
		require.Equal(t, "detachment from work and sleep", p.Query)
		require.Equal(t, CiteMarkers, p.Shape.Cite)
	})
	t.Run("a short reply with no topic does not retrieve", func(t *testing.T) {
		p := Decide(Analysis{Intent: "reflect", NeedsResearch: true}, true, "yes exactly", Situation{})
		require.False(t, p.Retrieve)
	})
	t.Run("listening never retrieves, whatever the topic", func(t *testing.T) {
		p := Decide(Analysis{Intent: "vent", NeedsResearch: true, Topic: "workload and long hours"}, true, "I am so tired of being ignored in every meeting", Situation{})
		require.Equal(t, StanceListen, p.Shape.Stance)
		require.False(t, p.Retrieve)
	})
}

func TestParseAnalysisDistrustsTheModel(t *testing.T) {
	an, err := parseAnalysis(`{"intent":"vent","needs_research":false,"search_query":"  ",
		"exhaustion":"high","cynicism":"extreme","reduced_efficacy":"not_evident",
		"demands":["workload","workload","astrology"],"resources":[],"valence":-4}`)
	require.NoError(t, err)
	require.Equal(t, "not_evident", an.Cynicism, "values outside the vocabulary become not_evident")
	require.Equal(t, []string{"workload"}, an.Demands, "unknown and repeated tags are dropped")
	require.Equal(t, -1.0, an.Valence, "valence is clamped")
	require.Equal(t, "", an.Topic)
	require.Equal(t, "", an.StanceHint, "a missing hint is no hint")
	require.Equal(t, "personal", an.Depth)
	require.Equal(t, "unclear", an.WantsAdvice, "a missing wants_advice is unclear, never a silent no")

	an, err = parseAnalysis(`{"intent":"vent","wants_advice":"no","stance_hint":"listen","depth":"heavy","topic":" motivation at work "}`)
	require.NoError(t, err)
	require.Equal(t, "no", an.WantsAdvice)
	require.Equal(t, "listen", an.StanceHint)
	require.Equal(t, "heavy", an.Depth)
	require.Equal(t, "motivation at work", an.Topic)
	an, err = parseAnalysis(`{"intent":"vent","stance_hint":"coach","depth":"deep"}`)
	require.NoError(t, err)
	require.Equal(t, "", an.StanceHint)
	require.Equal(t, "personal", an.Depth)
	an, err = parseAnalysis(`{"intent":"vent","wants_advice":"never"}`)
	require.NoError(t, err)
	require.Equal(t, "unclear", an.WantsAdvice)

	_, err = parseAnalysis(`{"intent":"diagnose"}`)
	require.Error(t, err, "an unknown intent is a failed analysis, not a guess")
	_, err = parseAnalysis(`not json`)
	require.Error(t, err)
}

func TestLevelKeepsMissingDistinctFromLow(t *testing.T) {
	require.Nil(t, level("not_evident"))
	require.Nil(t, level("nonsense"))
	require.Equal(t, int16(1), *level("low"))
	require.Equal(t, int16(3), *level("high"))
}

func TestRenumberCitations(t *testing.T) {
	text, used := renumberCitations("Sleep matters [3]. So does support [1, 3]. Made up [9].", 3)
	require.Equal(t, "Sleep matters [1]. So does support [2, 1]. Made up.", text)
	require.Equal(t, []int{3, 1}, used)

	text, used = renumberCitations("No excerpts were given [1].", 0)
	require.Equal(t, "No excerpts were given.", text)
	require.Empty(t, used)
}

func TestTopicIsCapped(t *testing.T) {
	long := strings.Repeat("burnout ", 200) // 1,600 runes
	an, err := parseAnalysis(`{"intent":"ask_evidence","needs_research":true,"topic":"` + long + `"}`)
	require.NoError(t, err)
	require.Len(t, []rune(an.Topic), maxQueryRunes)
}
