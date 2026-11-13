package registry

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEveryKindTheSchemaAllows(t *testing.T) {
	// games_kind_known in 0009
	require.Equal(t, []string{"constellations", "sudoku", "words"}, Kinds())
	for _, k := range Kinds() {
		e, ok := Get(k)
		require.True(t, ok)
		require.Equal(t, k, e.Kind())
	}
	_, ok := Get("chess")
	require.False(t, ok)
}
