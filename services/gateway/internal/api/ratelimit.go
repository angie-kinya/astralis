package api

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// A per-user token bucket held in memory.
type bucket struct {
	tokens   float64
	lastSeen time.Time
}

type rateLimiter struct {
	mu       sync.Mutex // guards buckets; every field below is read-only after construction
	buckets  map[string]*bucket
	rate     float64 // tokens restored per second
	capacity float64 // burst size
}

func newRateLimiter(perMinute int) *rateLimiter {
	if perMinute <= 0 {
		perMinute = 20
	}
	rl := &rateLimiter{
		buckets:  make(map[string]*bucket),
		rate:     float64(perMinute) / 60,
		capacity: float64(perMinute),
	}
	go rl.reap()
	return rl
}

// allow refills the caller's bucket for the time elapsed since they were last seen, then spends one token
func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[key]
	if !ok {
		b = &bucket{tokens: rl.capacity, lastSeen: now}
		rl.buckets[key] = b
	}

	b.tokens = min(rl.capacity, b.tokens+now.Sub(b.lastSeen).Seconds()*rl.rate)
	b.lastSeen = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// reap stops the map growing without bound. Without it, every user who ever signed in stays resident for the life of the process
func (rl *rateLimiter) reap() {
	for range time.Tick(10 * time.Minute) {
		rl.mu.Lock()
		for k, b := range rl.buckets {
			if time.Since(b.lastSeen) > time.Hour {
				delete(rl.buckets, k)
			}
		}
		rl.mu.Unlock()
	}
}

func (s *Server) rateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := currentUser(c)
		if !s.limiter.allow(u.ID) {
			slog.Warn("rate limited", "user_id", u.ID, "path", c.FullPath())
			c.Header("Retry-After", "60")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":   "rate_limited",
				"message": "You are sending messages faster than Astralis can think. Try again in a minute.",
			})
			return
		}
		c.Next()
	}
}
