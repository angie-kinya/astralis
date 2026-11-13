package api

import (
	"net/http"
	"regexp"
	"strings"
	"sync"

	"astralis/gateway/internal/agent"
	"astralis/gateway/internal/embed"
	"astralis/gateway/internal/store"

	"github.com/gin-gonic/gin"
)

// the eval harness (services/evals) replays scripted conversations through the
// real agent. this endpoint exists only in development: the router registers it
// only when ENV=development, and the handler refuses anywhere else as well

type devFact struct {
	Kind    string `json:"kind"`
	Subject string `json:"subject"`
	Detail  string `json:"detail"`
	Status  string `json:"status"`
}

type devTurnReq struct {
	Persona string    `json:"persona" binding:"required"`
	Message string    `json:"message" binding:"required"`
	Facts   []devFact `json:"facts"`
	History []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"history"`
	// continue a scenario; empty starts a fresh scratch user and session
	SessionID string `json:"sessionId"`
	// pin a voice variant for the session (the agent keeps a variant already set on a session)
	Variant string `json:"variant"`
}

// devRun is the slice of agent_runs the metrics need
type devRun struct {
	Route           string   `json:"route"`
	Intent          *string  `json:"intent"`
	PromptVariant   *string  `json:"promptVariant"`
	WordBudget      *int     `json:"wordBudget"`
	ReplyWords      *int     `json:"replyWords"`
	StyleViolations []string `json:"styleViolations"`
	Repaired        bool     `json:"repaired"`
	FactsUsed       int      `json:"factsUsed"`
	Moves           []string `json:"moves"`
	TotalMS         int      `json:"totalMs"`

	// v2.2: the turn plan, for stance accuracy, length in band, truncation
	// and research use
	Stance      *string `json:"stance"`
	BandMin     *int    `json:"bandMin"`
	BandMax     *int    `json:"bandMax"`
	Advice      *string `json:"advice"`
	CiteMode    *string `json:"citeMode"`
	Truncated   bool    `json:"truncated"`
	UsedInWords bool    `json:"usedInWords"`
	TopicTurns  int     `json:"topicTurns"`
	ChunksFound int     `json:"chunksFound"`
	ChunksCited int     `json:"chunksCited"`
	StopReason  *string `json:"stopReason"`
}

var personaRe = regexp.MustCompile(`^[a-z0-9_-]{1,40}$`)

// built once on first use; the server itself never needs an embedder
var (
	devEmbedOnce sync.Once
	devEmbedder  embed.Embedder
)

func (s *Server) handleDevTurn(c *gin.Context) {
	if s.cfg.Env != "development" {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"}) // defence in depth
		return
	}
	var req devTurnReq
	if err := c.ShouldBindJSON(&req); err != nil || !personaRe.MatchString(req.Persona) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "persona (a-z0-9_-) and message are required"})
		return
	}
	ctx := c.Request.Context()
	email := "eval+" + req.Persona + "@astralis.local"

	u, err := s.store.UserForVerifiedEmail(ctx, email, false)
	if err != nil {
		fail(c, err, "scratch user")
		return
	}
	sessionID := req.SessionID
	if sessionID == "" {
		// fresh scenario: erase the scratch user so facts, signals and trajectory
		// from an earlier run cannot leak into this one
		if err := s.store.DeleteUser(ctx, u.ID); err != nil {
			fail(c, err, "reset scratch user")
			return
		}
		if u, err = s.store.UserForVerifiedEmail(ctx, email, false); err != nil {
			fail(c, err, "scratch user")
			return
		}
		if _, err := s.store.GrantConsent(ctx, u.ID); err != nil {
			fail(c, err, "scratch consent")
			return
		}
		if sessionID, err = s.store.CreateSession(ctx, u.ID); err != nil {
			fail(c, err, "scratch session")
			return
		}
		if req.Variant != "" {
			_, _ = s.store.Pool().Exec(ctx, `UPDATE sessions SET prompt_variant=$2 WHERE id=$1`, sessionID, req.Variant)
		}
		if err := s.seedFacts(c, u.ID, req.Facts); err != nil {
			fail(c, err, "seed facts")
			return
		}
		for _, m := range req.History {
			if m.Role != "user" && m.Role != "assistant" {
				continue
			}
			if _, err := s.store.AddMessage(ctx, sessionID, m.Role, m.Content, false, 0); err != nil {
				fail(c, err, "seed history")
				return
			}
		}
	} else if ok, err := s.store.SessionBelongsTo(ctx, sessionID, u.ID); err != nil || !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown session for persona"})
		return
	}

	res, err := s.agent.Handle(ctx, agent.Turn{UserID: u.ID, SessionID: sessionID, Content: strings.TrimSpace(req.Message)})
	if err != nil {
		fail(c, err, "agent turn")
		return
	}

	var run *devRun
	r := devRun{}
	if err := s.store.Pool().QueryRow(ctx, `
		SELECT route, intent, prompt_variant, word_budget, reply_words, style_violations,
		       repaired, facts_used, moves, total_ms,
		       stance, band_min, band_max, advice, cite_mode, truncated, used_in_words, topic_turns,
		       chunks_found, chunks_cited, stop_reason
		FROM agent_runs WHERE session_id=$1 ORDER BY created_at DESC LIMIT 1`, sessionID).Scan(
		&r.Route, &r.Intent, &r.PromptVariant, &r.WordBudget, &r.ReplyWords, &r.StyleViolations,
		&r.Repaired, &r.FactsUsed, &r.Moves, &r.TotalMS,
		&r.Stance, &r.BandMin, &r.BandMax, &r.Advice, &r.CiteMode, &r.Truncated, &r.UsedInWords, &r.TopicTurns,
		&r.ChunksFound, &r.ChunksCited, &r.StopReason); err == nil {
		run = &r
	}
	c.JSON(http.StatusOK, gin.H{"sessionId": sessionID, "reply": res.Reply, "crisis": res.Crisis,
		"sources": res.Sources, "run": run})
}

// seedFacts embeds subject + detail so recall can find them; without an api key
// the facts still land, unembedded, and only the dated recall path sees them
func (s *Server) seedFacts(c *gin.Context, userID string, facts []devFact) error {
	if len(facts) == 0 {
		return nil
	}
	devEmbedOnce.Do(func() {
		if s.cfg.OpenAIAPIKey != "" {
			devEmbedder = embed.NewOpenAI(s.cfg.OpenAIAPIKey, s.cfg.EmbedModel, s.cfg.EmbedDim)
		}
	})
	var vecs [][]float32
	if devEmbedder != nil {
		texts := make([]string, len(facts))
		for i, f := range facts {
			texts[i] = f.Subject + ": " + f.Detail
		}
		vecs, _ = devEmbedder.Embed(c.Request.Context(), texts) // best effort
	}
	for i, f := range facts {
		var vec []float32
		if i < len(vecs) {
			vec = vecs[i]
		}
		if f.Status == "" {
			f.Status = "open"
		}
		err := s.store.UpsertFact(c.Request.Context(), userID, "",
			store.Fact{Kind: f.Kind, Subject: f.Subject, Detail: f.Detail, Status: f.Status}, vec)
		if err != nil {
			return err
		}
	}
	return nil
}
