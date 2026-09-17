package safety

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubClf struct {
	score float64
	err   error
}

func (s stubClf) Score(context.Context, string) (float64, error) { return s.score, s.err }

func TestRulesCatchObviousPhrasing(t *testing.T) {
	f := NewFilter(nil, 0.55)
	crisis := []string{
		"I want to kill myself",
		"lately I feel suicidal",
		"there is no reason to live anymore",
		"everyone would be better off dead without me",
	}
	for _, s := range crisis {
		require.True(t, f.Check(context.Background(), s).IsCrisis(), s)
	}
}

func TestOrdinaryBurnoutTalkIsNotFlagged(t *testing.T) {
	f := NewFilter(nil, 0.55)
	ok := []string{
		"I am completely exhausted by this job",
		"my manager keeps piling on work and I am dreading Mondays",
		"I killed that presentation today",
		"this workload is killing my motivation",
	}
	for _, s := range ok {
		require.False(t, f.Check(context.Background(), s).IsCrisis(), s)
	}
}

func TestClassifierLayer(t *testing.T) {
	ctx := context.Background()

	t.Run("above threshold flags", func(t *testing.T) {
		f := NewFilter(stubClf{score: 0.9}, 0.55)
		require.True(t, f.Check(ctx, "some subtle phrasing").IsCrisis())
	})

	t.Run("below threshold passes", func(t *testing.T) {
		f := NewFilter(stubClf{score: 0.1}, 0.55)
		require.False(t, f.Check(ctx, "just tired").IsCrisis())
	})

	t.Run("classifier error degrades to rules", func(t *testing.T) {
		f := NewFilter(stubClf{err: errors.New("down")}, 0.55)
		require.False(t, f.Check(ctx, "just tired").IsCrisis())
		require.True(t, f.Check(ctx, "I want to hurt myself").IsCrisis())
	})
}
