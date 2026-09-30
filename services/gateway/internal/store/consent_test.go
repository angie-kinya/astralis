package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConsentedSince(t *testing.T) {
	terms := time.Date(2026, time.September, 30, 0, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *time.Time { v := terms.Add(d); return &v }

	require.False(t, User{}.ConsentedSince(terms), "never consented")
	require.False(t, User{ConsentedAt: at(-time.Second)}.ConsentedSince(terms),
		"agreeing before the terms changed does not cover what changed")
	require.True(t, User{ConsentedAt: at(0)}.ConsentedSince(terms), "the exact moment counts")
	require.True(t, User{ConsentedAt: at(48 * time.Hour)}.ConsentedSince(terms))
}
