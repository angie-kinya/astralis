// Package auth holds the primitives for email sign-in: normalising an address,
// generating and hashing one-time codes, and generating and hashing session
// tokens. It has no idea HTTP or a database exist — which is what makes every
// function here trivially testable.
//
// The flow it supports:
//
//	email  ->  6-digit code sent to that inbox  ->  code checked  ->  session
//
// Proving ownership of the inbox is the whole point. An email typed into a box
// and trusted is not an identity: anyone could type yours and read your
// conversations.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/mail"
	"strings"
	"time"
)

const (
	// CookieName is the one cookie Astralis sets. HttpOnly, so no script —
	// including one injected by a bug — can read it.
	CookieName = "astralis_session"

	CodeTTL        = 10 * time.Minute
	MaxCodeGuesses = 5
	// Codes requested per address per hour. Stops one person using the form
	// to flood someone else's inbox.
	MaxCodesPerHour = 5
	SessionTTL      = 30 * 24 * time.Hour
)

var ErrInvalidEmail = errors.New("that does not look like an email address")

// NormalizeEmail lowercases and validates. Lowercasing matters: without it,
// "Angie@x.com" and "angie@x.com" become two accounts, and the person who
// signs in with the second one finds their history missing.
func NormalizeEmail(raw string) (string, error) {
	e := strings.ToLower(strings.TrimSpace(raw))
	if len(e) < 3 || len(e) > 254 {
		return "", ErrInvalidEmail
	}
	addr, err := mail.ParseAddress(e)
	// ParseAddress also accepts "Name <a@b.c>". Require the bare address back,
	// so a display name can never smuggle itself into the identity column.
	if err != nil || addr.Address != e || !strings.Contains(e[strings.LastIndex(e, "@")+1:], ".") {
		return "", ErrInvalidEmail
	}
	return e, nil
}

// NewCode returns six uniformly random digits from crypto/rand.
//
// Not math/rand: its output is predictable from a few samples, and a
// predictable sign-in code is no code at all. rand.Int also avoids the modulo
// bias you would get from taking random bytes % 1_000_000.
func NewCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// HashCode is an HMAC over the address and code, keyed by the server secret.
// Binding the address in means a code issued to one person can never verify
// another; keying it means a copy of the database contains no usable codes.
func HashCode(secret, email, code string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(email))
	m.Write([]byte{0}) // separator: "ab"+"c" must not collide with "a"+"bc"
	m.Write([]byte(code))
	return hex.EncodeToString(m.Sum(nil))
}

// NewToken returns 256 random bits, URL-safe. This goes in the cookie.
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken is what the database stores. A plain SHA-256 is right here, unlike
// for passwords: the input is 256 random bits, so there is nothing to
// brute-force and no need for a slow hash.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// MaskEmail is the only form of an address that may appear in a log line.
// "angie@gmail.com" -> "a***@gmail.com".
func MaskEmail(email string) string {
	at := strings.LastIndex(email, "@")
	if at < 1 {
		return "***"
	}
	return email[:1] + "***" + email[at:]
}
