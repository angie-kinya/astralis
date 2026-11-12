package agent

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecideFallsBackToPlainRetrievalWhenAnalysisFails(t *testing.T) {
	p := Decide(Analysis{}, false, "my manager keeps moving deadlines", Situation{})
	require.True(t, p.Retrieve)
	require.Equal(t, "my manager keeps moving deadlines", p.Query)
	require.Empty(t, p.Guidance)
}

func TestDecideRoutesOnIntent(t *testing.T) {
	t.Run("greeting never retrieves", func(t *testing.T) {
		p := Decide(Analysis{Intent: "greeting", NeedsResearch: true}, true, "hello there", Situation{})
		require.False(t, p.Retrieve)
		require.NotEmpty(t, p.Guidance)
	})
	t.Run("asking for evidence always retrieves, with the rewritten query", func(t *testing.T) {
		p := Decide(Analysis{Intent: "ask_evidence", SearchQuery: "detachment from work and sleep"}, true, "does that help?", Situation{})
		require.True(t, p.Retrieve)
		require.Equal(t, "detachment from work and sleep", p.Query)
	})
	t.Run("a short reply without a rewritten query does not retrieve", func(t *testing.T) {
		p := Decide(Analysis{Intent: "vent", NeedsResearch: true}, true, "yes exactly", Situation{})
		require.False(t, p.Retrieve)
	})
	t.Run("venting retrieves only when research would help", func(t *testing.T) {
		p := Decide(Analysis{Intent: "vent", NeedsResearch: false}, true, "I am so tired of being ignored in every meeting", Situation{})
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
	require.Equal(t, "", an.SearchQuery)

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

func TestSearchQueryIsCapped(t *testing.T) {
	long := strings.Repeat("burnout ", 200) // 1,600 runes
	an, err := parseAnalysis(`{"intent":"ask_evidence","needs_research":true,"search_query":"` + long + `"}`)
	require.NoError(t, err)
	require.Len(t, []rune(an.SearchQuery), maxQueryRunes)
}
