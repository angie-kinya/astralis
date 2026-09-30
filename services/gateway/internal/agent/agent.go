// Package agent runs one chat turn as a pipeline of separate stages:
//
//	guard -> remember -> analyse -> plan -> act -> respond -> verify -> record
//
// Each stage has one job. Only analyse and respond call a language model, and
// they call different ones: a small fast model reads the message, a larger
// one writes the reply. Everything between them is ordinary Go, so routing
// decisions are deterministic and testable, and a failure in any optional
// stage degrades the turn instead of breaking it
//
// Nothing here trains a model. The agent measures (turn_signals) and records
// what it did (agent_runs); a separate prediction model can learn from those
// tables later, in Python, with no change to this package
package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"astralis/gateway/internal/embed"
	"astralis/gateway/internal/llm"
	"astralis/gateway/internal/safety"
	"astralis/gateway/internal/store"
)

// Model is the one method the agent needs from a language model client
type Model interface {
	Generate(ctx context.Context, r llm.Request) (llm.Response, error)
}

type Config struct {
	// analyst model reads each message, small and fast, and runs on every turn
	AnalystModel  string
	RespondEffort string
	TopK          int
	//the floor below which a passage is not relevant enough to put in front of the model:
	MinSimilarity  float64
	AnalyseTimeout time.Duration
}

type Agent struct {
	store    *store.Store
	embedder embed.Embedder
	model    Model
	filter   *safety.Filter
	cfg      Config
}

func New(s *store.Store, e embed.Embedder, m Model, f *safety.Filter, cfg Config) *Agent {
	if cfg.TopK <= 0 {
		cfg.TopK = 5
	}
	if cfg.AnalyseTimeout <= 0 {
		cfg.AnalyseTimeout = 8 * time.Second
	}
	return &Agent{store: s, embedder: e, model: m, filter: f, cfg: cfg}
}

// a message from one user in one of their sessions:
type Turn struct {
	UserID    string
	SessionID string
	Content   string
}

type Result struct {
	Reply            string
	Crisis           bool
	RecommendationID string
	// Sources are the passages the reply cited, in citation order
	Sources []store.Chunk
}

// Handle runs the whole pipeline for one turn
func (a *Agent) Handle(ctx context.Context, t Turn) (Result, error) {
	start := time.Now()
	run := store.Run{UserID: t.UserID, SessionID: t.SessionID, Route: "failed", StagesMS: map[string]int64{}}
	stage := func(name string, since time.Time) { run.StagesMS[name] = time.Since(since).Milliseconds() }
	note := func(stage string, err error) {
		msg := stage + ": " + err.Error()
		if run.Error != "" {
			msg = run.Error + "; " + msg
		}
		run.Error = truncate(msg, 600)
	}

	// the trace is written whatever happens, including failures, and on a
	// context of its own so a closed browser tab cannot cancel the audit
	defer func() {
		run.TotalMS = time.Since(start).Milliseconds()
		rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		if err := a.store.SaveRun(rctx, run); err != nil {
			slog.Error("save agent run", "err", err)
		}
	}()

	// GUARD. before anything is generated, stored for memory, or retrieved
	t0 := time.Now()
	verdict := a.filter.Check(ctx, t.Content)
	stage("guard", t0)
	run.SafetyRule = verdict.MatchedRule

	userMsgID, err := a.store.AddMessage(ctx, t.SessionID, "user", t.Content, verdict.IsCrisis(), verdict.Score)
	if err != nil {
		note("persist", err)
		return Result{}, fmt.Errorf("save message: %w", err)
	}
	run.UserMessageID = userMsgID

	if verdict.IsCrisis() {
		return a.crisis(ctx, t, userMsgID, verdict, &run), nil
	}

	// REMEMBER. the recent window of this session, and the gist of earlier ones
	t0 = time.Now()
	history, err := a.store.RecentMessages(ctx, t.SessionID, 12)
	if err != nil {
		note("memory", err)
		return Result{}, fmt.Errorf("load history: %w", err)
	}
	summaries, err := a.store.RecentSummaries(ctx, t.UserID, 3)
	if err != nil {
		note("memory", err) // continuity is nice to have, not a reason to fail
	}
	stage("memory", t0)

	// ANALYSE. bounded by its own timeout; failure means "no analysis"
	t0 = time.Now()
	actx, cancel := context.WithTimeout(ctx, a.cfg.AnalyseTimeout)
	an, usage, aerr := a.analyse(actx, tail(history, 6))
	cancel()
	stage("analyse", t0)
	run.InputTokens += usage.InputTokens
	run.OutputTokens += usage.OutputTokens
	if aerr != nil {
		note("analyse", aerr)
	} else {
		run.AnalyserOK, run.Intent = true, an.Intent
	}

	// PLAN. pure Go
	plan := Decide(an, aerr == nil, t.Content)

	// ACT. only the tools the plan asked for
	var chunks []store.Chunk
	if plan.Retrieve {
		t0 = time.Now()
		run.RetrievalUsed = true
		if chunks, err = a.retrieve(ctx, plan.Query); err != nil {
			note("retrieve", err) // answer ungrounded rather than not at all
		}
		stage("retrieve", t0)
	}
	run.ChunksFound = len(chunks)
	for i, c := range chunks {
		if i == 0 || c.Similarity > *run.TopSimilarity {
			sim := c.Similarity
			run.TopSimilarity = &sim
		}
	}

	// RESPOND
	t0 = time.Now()
	res, err := a.model.Generate(ctx, llm.Request{
		System:    buildSystem(summaries, chunks, plan),
		Messages:  conversation(history, t.Content),
		MaxTokens: 2048,
		Effort:    a.cfg.RespondEffort,
	})
	stage("respond", t0)
	run.InputTokens += res.Usage.InputTokens
	run.OutputTokens += res.Usage.OutputTokens
	run.StopReason = res.StopReason
	if err == nil && strings.TrimSpace(res.Text) == "" {
		err = fmt.Errorf("empty reply (stop_reason=%s)", res.StopReason)
	}
	if err != nil {
		note("respond", err)
		return Result{}, fmt.Errorf("generate reply: %w", err)
	}

	// VERIFY. keep only citations that point at something real
	text, cited := renumberCitations(res.Text, len(chunks))
	sources := make([]store.Chunk, 0, len(cited))
	citedIDs := make([]string, 0, len(cited))
	for _, k := range cited {
		sources = append(sources, chunks[k-1])
		citedIDs = append(citedIDs, chunks[k-1].ID)
	}
	run.ChunksCited = len(cited)

	// RECORD
	t0 = time.Now()
	replyID, err := a.store.AddMessage(ctx, t.SessionID, "assistant", text, false, 0)
	if err != nil {
		note("persist", err)
		return Result{}, fmt.Errorf("save reply: %w", err)
	}
	run.ReplyMessageID = replyID
	run.Route = "agent"

	allIDs := make([]string, len(chunks))
	for i, c := range chunks {
		allIDs[i] = c.ID
	}
	recID, err := a.store.SaveRecommendation(ctx, t.SessionID, replyID, allIDs, citedIDs)
	if err != nil {
		note("record", err)
	}
	if run.AnalyserOK {
		if err := a.store.SaveSignals(ctx, toSignals(an, t, userMsgID)); err != nil {
			note("record", err)
		}
	}
	_ = a.store.IncrementRetrieved(ctx, allIDs)
	_ = a.store.TouchSession(ctx, t.SessionID)
	// the first user message doubles as the sidebar title
	if n, _ := a.store.CountMessages(ctx, t.SessionID); n <= 2 {
		_ = a.store.SetSessionTitleIfEmpty(ctx, t.SessionID, truncate(t.Content, 60))
	}
	stage("record", t0)

	return Result{Reply: text, RecommendationID: recID, Sources: sources}, nil
}

// crisis answers with fixed, human written text. the model is never called,
// and because both rows are stored flagged, neither the prompt window nor
// the session summary will ever contain them
func (a *Agent) crisis(ctx context.Context, t Turn, msgID string, v safety.Result, run *store.Run) Result {
	run.Route = "crisis"
	if err := a.store.FlagCrisis(ctx, msgID, v.MatchedRule, v.Score); err != nil {
		slog.Error("flag crisis", "err", err) // audit failure must not swallow the response
	}
	if id, err := a.store.AddMessage(ctx, t.SessionID, "assistant", safety.CrisisResponse, true, v.Score); err != nil {
		slog.Error("save crisis reply", "err", err)
	} else {
		run.ReplyMessageID = id
	}
	return Result{Reply: safety.CrisisResponse, Crisis: true, Sources: []store.Chunk{}}
}

func (a *Agent) retrieve(ctx context.Context, query string) ([]store.Chunk, error) {
	vecs, err := a.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 || len(vecs[0]) == 0 {
		return nil, errors.New("embedder returned no vector")
	}
	return a.store.SearchChunks(ctx, vecs[0], a.cfg.TopK, a.cfg.MinSimilarity)
}

// conversation turns stored history into model messages. the handler has
// already persisted the user's turn, so it is normally the last entry; it is
// appended only if it is missing
func conversation(history []store.Message, userMsg string) []llm.Message {
	msgs := make([]llm.Message, 0, len(history)+1)
	for _, m := range history {
		if m.Role == "system" {
			continue
		}
		msgs = append(msgs, llm.Message{Role: m.Role, Content: m.Content})
	}
	if n := len(msgs); n == 0 || msgs[n-1].Role != "user" || msgs[n-1].Content != userMsg {
		msgs = append(msgs, llm.Message{Role: "user", Content: userMsg})
	}
	return msgs
}

func tail(msgs []store.Message, n int) []store.Message {
	if len(msgs) <= n {
		return msgs
	}
	return msgs[len(msgs)-n:]
}

// rune aware, so a cut never lands in the middle of a character
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
