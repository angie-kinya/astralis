// Package api wires HTTP routes to handlers. Handlers stay thin: parse input,
// call a service, shape the response. Business logic lives in agent/store/safety
// so it can be tested without spinning up a server.
package api

import (
	"log/slog"
	"net/http"

	"astralis/gateway/internal/agent"
	"astralis/gateway/internal/config"
	"astralis/gateway/internal/mail"
	"astralis/gateway/internal/safety"
	"astralis/gateway/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
)

type Server struct {
	cfg     *config.Config
	store   *store.Store
	agent   *agent.Agent
	filter  *safety.Filter
	queue   *asynq.Client // nil when REDIS_ADDR is unset; see handleEndSession
	mailer  mail.Sender
	limiter *rateLimiter
	// A separate bucket for sign-in, keyed by IP. Sharing the per-user limiter
	// would let a sign-in flood from one address eat into chat allowances.
	authLimiter *rateLimiter
}

func NewServer(cfg *config.Config, s *store.Store, ag *agent.Agent, f *safety.Filter,
	q *asynq.Client, m mail.Sender) *Server {
	return &Server{
		cfg: cfg, store: s, agent: ag, filter: f, queue: q, mailer: m,
		limiter:     newRateLimiter(cfg.RateLimitPerMin),
		authLimiter: newRateLimiter(cfg.AuthRateLimitPerMin),
	}
}

func (s *Server) Router() *gin.Engine {
	if s.cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	// gin.New(), not gin.Default(): Default installs Gin's own logger, which prints the full path of every request.
	r := gin.New()
	r.Use(gin.Recovery(), requestLogger())

	// Only Caddy (or the Next dev server) may tell us the client's real IP.
	// Trusting every X-Forwarded-For would let anyone bypass the per-IP
	// sign-in limit by inventing a header.
	if err := r.SetTrustedProxies(s.cfg.TrustedProxies); err != nil {
		slog.Error("trusted proxies", "err", err)
	}

	// No CORS middleware, deliberately. The browser only ever talks to one
	// origin: Caddy routes /api/* to this service in production, and the Next
	// dev server rewrites /api/* here locally. Same-origin means there is no
	// cross-origin policy to get wrong, and the session cookie is first-party.

	// Liveness only. Render restarts a container whose health check fails
	// if this also pinged Postgres, a ten-second database blip would become a restart loop.
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Sign-in. The only endpoints reachable without a session, so they carry
	// their own per-IP limit.
	pub := r.Group("/api/v1/auth", s.authRateLimit())
	{
		pub.POST("/start", s.handleAuthStart)
		pub.POST("/verify", s.handleAuthVerify)
		pub.POST("/signout", s.handleSignOut)
	}

	// Middleware composes left to right. By the time any handler below runs,
	// the session is valid and a real, email-verified user row exists.
	v1 := r.Group("/api/v1", s.authenticate())
	{
		v1.GET("/me", s.handleMe)
		v1.POST("/consent", s.handleConsent)
		v1.DELETE("/me", s.handleDeleteMe)
		v1.POST("/me/notify", s.handleNotifyReleases)

		// Onboarding order is email -> name -> consent -> Astralis, so choosing
		// a handle does NOT require consent. A handle is not conversation
		// content; the consent sheet is about what you then say.
		v1.POST("/me/handle", s.handleSetHandle)
		v1.POST("/me/peer-opt-in", s.requireConsent(), s.handlePeerOptIn)

		// Operations. Aggregates and ids only, never message text. Anyone not
		// on the admin list gets a 404, so the routes do not reveal themselves.
		admin := v1.Group("/admin", s.requireAdmin())
		admin.GET("/overview", s.handleAdminOverview)
		admin.GET("/runs", s.handleAdminRuns)

		// Everything that stores conversation content sits behind consent as a rate limit.
		// Enforcing it here structurally beats trusting six handlers to each remember.
		c := v1.Group("", s.requireConsent(), s.rateLimit())
		c.GET("/sessions", s.handleListSessions)
		c.POST("/sessions", s.handleCreateSession)
		c.GET("/sessions/:id/messages", s.handleGetMessages)
		c.POST("/sessions/:id/messages", s.handleSendMessage)
		c.POST("/sessions/:id/end", s.handleEndSession)
		c.POST("/feedback", s.handleFeedback)

		// The circle. Peer messages run through the same safety filter as the
		// agent path -- see handleSendPeerMessage for why that matters more
		// here rather than less.
		c.GET("/peers", s.handleListPeers)
		c.GET("/peers/unread", s.handleUnread)
		c.GET("/peers/:id/messages", s.handleGetConversation)
		c.POST("/peers/:id/messages", s.handleSendPeerMessage)

		// Notes never reach a model, an embedder, or the retrieval index.
		c.GET("/notes", s.handleListNotes)
		c.POST("/notes", s.handleCreateNote)
		c.PUT("/notes/:id", s.handleSaveNote)
		c.DELETE("/notes/:id", s.handleDeleteNote)
	}
	return r
}
