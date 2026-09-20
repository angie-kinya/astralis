// Package api wires HTTP routes to handlers. Handlers stay thin: parse input,
// call a service, shape the response. Business logic lives in rag/store/safety
// so it can be tested without spinning up a server.
package api

import (
	"net/http"
	"time"

	"astralis/gateway/internal/auth"
	"astralis/gateway/internal/config"
	"astralis/gateway/internal/rag"
	"astralis/gateway/internal/safety"
	"astralis/gateway/internal/store"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
)

type Server struct {
	cfg     *config.Config
	store   *store.Store
	engine  *rag.Engine
	filter  *safety.Filter
	queue   *asynq.Client
	limiter *rateLimiter
}

func NewServer(cfg *config.Config, s *store.Store, e *rag.Engine, f *safety.Filter, q *asynq.Client) *Server {
	return &Server{
		cfg: cfg, store: s, engine: e, filter: f, queue: q,
		limiter: newRateLimiter(cfg.RateLimitPerMin),
	}
}

func (s *Server) Router() *gin.Engine {
	if s.cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	// gin.New(), not gin.Default(): Default installs Gin's own logger, which prints the full path of every request.
	r := gin.New()
	r.Use(gin.Recovery(), requestLogger())

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{s.cfg.CORSOrigin},
		AllowMethods:     []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Liveness only. Render restarts a container whose health check fails
	// if this also pinged Postgres, a ten-second database blip would become a restart loop.
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Middleware composes left to right. By the time any handler below runs, the token is verified and a real user row exists.
	v1 := r.Group("/api/v1", auth.Middleware(s.cfg.BackendJWTSecret), s.resolveUser())
	{
		v1.GET("/me", s.handleMe)
		v1.POST("/consent", s.handleConsent)
		v1.DELETE("/me", s.handleDeleteMe)

		// Onboarding. Behind consent but NOT behind a handle, because this is
		// the endpoint that creates the handle.
		v1.POST("/me/handle", s.requireConsent(), s.handleSetHandle)
		v1.POST("/me/peer-opt-in", s.requireConsent(), s.handlePeerOptIn)

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
