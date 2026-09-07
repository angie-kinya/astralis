// Package auth verifies the short-lived HS256 token minted by the Next.js
// layer after Auth.js has established a session.
//
// Why two tokens? Auth.js issues an encrypted JWE session cookie that Go
// cannot read. So the browser asks Next.js for a plain signed JWT and sends
// *that* to the Go API. Next.js owns login; Go owns the API. Neither knows
// the other's internals — only the shared secret and the claim shape.
package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	jwt.RegisteredClaims
}

const ctxKey = "astralis_claims"

// middleware will reject anything without a valid bearere token
func Middleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if raw == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}
		claims, err := Parse(raw, secret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set(ctxKey, claims)
		c.Next()
	}
}

func Parse(raw, secret string) (*Claims, error) {
	claims := &Claims{}
	tok, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		// Pin the algorithm. Without this check an attacker can present a
		// token signed with "none" and walk straight in.
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if !tok.Valid {
		return nil, errors.New("token invalid")
	}
	if claims.Subject == "" {
		return nil, errors.New("token missing subject")
	}
	return claims, nil
}

func From(c *gin.Context) *Claims {
	v, ok := c.Get(ctxKey)
	if !ok {
		return nil
	}
	return v.(*Claims)
}
