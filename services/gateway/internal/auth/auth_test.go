package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func sign(t *testing.T, secret string, c Claims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte(secret))
	require.NoError(t, err)
	return s
}

// table-driven tests for one loop, many cases, and clear names
func TestParse(t *testing.T) {
	const secret = "test-secret"
	valid := Claims{
		Email: "a@b.com",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	expired := valid
	expired.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour))
	noSub := valid
	noSub.Subject = ""

	cases := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{"valid", sign(t, secret, valid), false},
		{"wrong secret", sign(t, "other-secret", valid), true},
		{"expired", sign(t, secret, expired), true},
		{"missing subject", sign(t, secret, noSub), true},
		{"garbage", "not-a-jwt", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.token, secret)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "user-1", got.Subject)
		})
	}
}
