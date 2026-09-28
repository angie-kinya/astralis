package auth

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeEmail(t *testing.T) {
	ok := map[string]string{
		"angie@example.com":        "angie@example.com",
		"  Angie@Example.COM  ":    "angie@example.com",
		"first.last+tag@uni.ac.ke": "first.last+tag@uni.ac.ke",
	}
	for in, want := range ok {
		got, err := NormalizeEmail(in)
		require.NoError(t, err, in)
		require.Equal(t, want, got)
	}

	bad := []string{
		"", "angie", "angie@", "@example.com", "angie@localhost",
		"Angie <angie@example.com>", // display names must not reach the identity column
		strings.Repeat("a", 250) + "@x.co",
	}
	for _, in := range bad {
		_, err := NormalizeEmail(in)
		require.ErrorIs(t, err, ErrInvalidEmail, in)
	}
}

func TestNewCodeIsSixDigits(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		c, err := NewCode()
		require.NoError(t, err)
		require.Len(t, c, 6)
		for _, r := range c {
			require.True(t, r >= '0' && r <= '9', c)
		}
		seen[c] = true
	}
	// 200 draws from a million should essentially never repeat much. This is
	// a smoke check that we are not returning a constant, not a statistics test.
	require.Greater(t, len(seen), 190)
}

func TestHashCodeBindsEmailAndSecret(t *testing.T) {
	h := HashCode("s1", "a@x.com", "123456")
	require.Equal(t, h, HashCode("s1", "a@x.com", "123456"), "deterministic")
	require.NotEqual(t, h, HashCode("s1", "b@x.com", "123456"), "a code for one address must not verify another")
	require.NotEqual(t, h, HashCode("s2", "a@x.com", "123456"), "a different secret must give a different hash")
	require.NotEqual(t, HashCode("s", "ab", "c"), HashCode("s", "a", "bc"), "separator prevents boundary collisions")
}

func TestTokens(t *testing.T) {
	a, err := NewToken()
	require.NoError(t, err)
	b, _ := NewToken()
	require.NotEqual(t, a, b)
	require.Len(t, HashToken(a), 64)
	require.NotEqual(t, a, HashToken(a), "the stored form must not be the cookie value")
}

func TestMaskEmail(t *testing.T) {
	require.Equal(t, "a***@gmail.com", MaskEmail("angie@gmail.com"))
	require.Equal(t, "***", MaskEmail("nonsense"))
}
