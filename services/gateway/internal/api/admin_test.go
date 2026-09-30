package api

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsAdmin(t *testing.T) {
	// The same digest `printf '%s' owner@example.com | sha256sum` prints.
	sum := sha256.Sum256([]byte("owner@example.com"))
	hashes := []string{hex.EncodeToString(sum[:])}

	require.True(t, isAdmin("owner@example.com", hashes))
	require.True(t, isAdmin("  Owner@Example.com ", hashes), "case and spaces do not matter")
	require.False(t, isAdmin("someone@example.com", hashes))
	require.False(t, isAdmin("owner@example.com", nil), "no list means no admins")
}

func TestBoundedInt(t *testing.T) {
	require.Equal(t, 50, boundedInt("", 50, 1, 500))
	require.Equal(t, 500, boundedInt("100000", 50, 1, 500))
	require.Equal(t, 1, boundedInt("-3", 50, 1, 500))
}