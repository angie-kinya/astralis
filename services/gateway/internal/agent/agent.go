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
// v2 moves behaviour out of the prompt and into Go, and knowledge about the
// person out of prose and into rows. the prompt says who Astralis is
// (character.go); code decides how long it speaks and whether it asks
// (shape.go), what it recalls (person_facts, trajectory), and whether the
// reply passes (lint.go, with one repair pass)
//
// v2.2 gives each turn one purpose. Go picks a stance (stance.go: listen,
// explore, advise, inform) from the analysis and the thread the session is
// on (topic.go), and the stance carries the length band, the questions, the
// advice and how research is used. the token limit is a safety ceiling only,
// and no reply is ever stored or shown cut off mid-sentence (finish.go)
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
	"slices"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

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
	// Repair allows one revision pass when the first draft fails the style
	// lint. off = log-only mode: violations are still recorded on every run
	Repair bool
	// Variants are the voices the bandit chooses between. empty = the default only
	Variants []string
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
	// an unknown name would silently become the base voice; drop it instead
	known := cfg.Variants[:0:0]
	for _, v := range cfg.Variants {
		if _, ok := Variants[v]; ok {
			known = append(known, v)
		}
	}
	if len(known) == 0 {
		known = []string{DefaultVariant}
	}
	cfg.Variants = known
	return &Agent{store: s, embedder: e, model: m, filter: f, cfg: cfg}
}

// a message from one user in one of their sessions:
type Turn struct {
	UserID    string
	SessionID string
	Content   string
	// Progress receives truthful status lines ("reading 2 studies") while the
	// turn runs. called synchronously, so it must not block; nil is fine
	Progress func(status string)
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

	// REMEMBER. the recent window of this session is one fast query, and
	// everything after it reads independently
	t0 = time.Now()
	history, err := a.store.RecentMessages(ctx, t.SessionID, 12)
	if err != nil {
		note("memory", err)
		return Result{}, fmt.Errorf("load history: %w", err)
	}
	stage("history", t0)

	// ANALYSE + RECALL, in parallel. each goroutine writes only its own
	// variables, so there is no shared write and no mutex. optional reads
	// swallow their own errors, so one failure never cancels the others, and
	// pre-reply latency is the slowest read rather than the sum of them
	var (
		an         Analysis
		usage      llm.Usage
		aerr       error
		qvec       []float32
		facts      []store.Fact
		pattern    store.Pattern
		moved      map[string]bool
		topic      store.TopicState
		topicErr   error
		summaries  []string
		variant    string
		msAnalyse  int64
		msRecall   int64
		recallErr  error
		patternErr error
		summaryErr error
	)
	t0 = time.Now()
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { // analyse, bounded by its own timeout; failure means "no analysis"
		s := time.Now()
		actx, cancel := context.WithTimeout(gctx, a.cfg.AnalyseTimeout)
		defer cancel()
		an, usage, aerr = a.analyse(actx, tail(history, 6))
		msAnalyse = time.Since(s).Milliseconds()
		return nil
	})
	g.Go(func() error { // embed once, recall facts; the vector is reused for research
		s := time.Now()
		defer func() { msRecall = time.Since(s).Milliseconds() }()
		v, err := a.embedder.Embed(gctx, []string{t.Content})
		if err != nil || len(v) == 0 || len(v[0]) == 0 {
			recallErr = errors.Join(err, errors.New("no vector"))
			return nil
		}
		qvec = v[0]
		facts, recallErr = a.store.RecallFacts(gctx, t.UserID, qvec, 4, 0.30)
		return nil
	})
	g.Go(func() error {
		pattern, patternErr = a.store.Pattern(gctx, t.UserID)
		if patternErr == nil {
			moved, patternErr = a.store.SessionMoves(gctx, t.SessionID)
		}
		return nil
	})
	g.Go(func() error {
		summaries, summaryErr = a.store.RecentSummaries(gctx, t.UserID, 2)
		return nil
	})
	g.Go(func() error { topic, topicErr = a.store.LastTopic(gctx, t.SessionID); return nil })
	g.Go(func() error { variant = a.variant(gctx, t.SessionID); return nil })
	_ = g.Wait()
	stage("gather", t0)
	run.StagesMS["analyse"], run.StagesMS["recall"] = msAnalyse, msRecall
	for name, err := range map[string]error{"recall": recallErr, "pattern": patternErr, "memory": summaryErr, "topic": topicErr} {
		if err != nil {
			note(name, err) // continuity is nice to have, not a reason to fail
		}
	}
	run.InputTokens += usage.InputTokens
	run.OutputTokens += usage.OutputTokens
	if aerr != nil {
		note("analyse", aerr)
	} else {
		run.AnalyserOK, run.Intent = true, an.Intent
	}
	run.PromptVariant = variant

	// PLAN. pure Go
	prev := assistantReplies(history)
	asked := make([]bool, len(prev))
	for i, r := range prev {
		asked[i] = askedQuestion(r)
	}
	plan := Decide(an, aerr == nil, t.Content, Situation{
		Asked:     asked,
		Pattern:   pattern,
		Moved:     moved,
		Variant:   variant,
		Topic:     topic,
		Earlier:   earlierUser(history, t.Content),
		LastReply: lastOf(prev),
	})
	run.Moves = plan.Moves
	run.Stance, run.Advice = string(plan.Shape.Stance), plan.Shape.Advice.String()
	run.BandMin, run.BandMax = plan.Shape.Band.Min, plan.Shape.Band.Max
	run.Topic, run.TopicTurns = plan.Topic.Topic, plan.Topic.Turns

	// ACT. the topic is embedded once and used twice: to search the research,
	// and to recall what they said before about this thread, which the
	// message alone ("it's more that nothing feels worth starting") may not reach
	var tvec []float32
	if tp := plan.Topic.Topic; tp != "" && tp != t.Content {
		t0 = time.Now()
		v, err := a.embedder.Embed(ctx, []string{tp})
		if err != nil || len(v) == 0 || len(v[0]) == 0 {
			note("topic", errors.Join(err, errors.New("no topic vector")))
		} else {
			tvec = v[0]
			more, err := a.store.RecallFacts(ctx, t.UserID, tvec, 4, 0.30)
			if err != nil {
				note("recall", err)
			}
			facts = mergeFacts(facts, more, maxFacts)
		}
		stage("topic", t0)
	}
	run.FactsUsed = len(facts)
	if len(facts) > 0 {
		progress(t, "checking what you told me before")
	}

	// only the tools the plan asked for
	var chunks []store.Chunk
	if plan.Retrieve {
		t0 = time.Now()
		run.RetrievalUsed = true
		var vec []float32 // nil asks retrieve to embed the query itself
		switch plan.Query {
		case t.Content:
			vec = qvec
		case plan.Topic.Topic:
			vec = tvec
		}
		if chunks, err = a.retrieve(ctx, plan.Query, vec); err != nil {
			note("retrieve", err) // answer ungrounded rather than not at all
		}
		stage("retrieve", t0)
		switch n := len(chunks); {
		case n == 1:
			progress(t, "reading a study")
		case n > 1:
			progress(t, fmt.Sprintf("reading %d studies", n))
		}
	}
	if len(chunks) == 0 {
		plan.Shape.Cite = CiteNone // nothing to draw on, so say nothing about research
	}
	run.CiteMode = plan.Shape.Cite.String()
	run.ChunksFound = len(chunks)
	run.WordBudget = plan.Shape.Band.Max
	for i, c := range chunks {
		if i == 0 || c.Similarity > *run.TopSimilarity {
			sim := c.Similarity
			run.TopSimilarity = &sim
		}
	}

	// RESPOND
	t0 = time.Now()
	system := buildSystem(variant, summaries, facts, chunks, plan)
	msgs := conversation(history, t.Content)
	d, err := a.respond(ctx, system, msgs, &run)
	stage("respond", t0)
	if err != nil {
		note("respond", err)
		return Result{}, fmt.Errorf("generate reply: %w", err)
	}

	// VERIFY. the style lint first, with one repair pass at most: a second
	// failure ships with its violations logged, which is what evals are for.
	// a draft the ceiling cut fails the lint as truncated, so the repair is
	// also the first attempt to finish it properly
	run.StyleViolations = lintDraft(d, plan.Shape, prev)
	if len(run.StyleViolations) > 0 && a.cfg.Repair {
		t0 = time.Now()
		revised, rerr := a.respond(ctx, system+repairNote(run.StyleViolations, d.text), msgs, &run)
		stage("repair", t0)
		if rerr != nil {
			note("repair", rerr) // the first draft still stands
		} else if len(lintDraft(revised, plan.Shape, prev)) < len(run.StyleViolations) {
			d, run.Repaired = revised, true
		}
	}
	// then the last word on truncation: whatever was cut loses its unfinished
	// sentence, and a reply with no finished sentence at all is a failure
	// rather than something to show
	final, ok := finish(d.text, d.cut)
	if !ok {
		err := errors.New("reply cut off before its first sentence ended")
		note("respond", err)
		return Result{}, fmt.Errorf("generate reply: %w", err)
	}
	run.Truncated = d.cut
	// then citations: keep only markers that point at something real. outside
	// the markers mode every marker goes, whatever the model wrote
	available := len(chunks)
	if plan.Shape.Cite != CiteMarkers {
		available = 0
	}
	text, cited := renumberCitations(final, available)
	sources := make([]store.Chunk, 0, len(cited))
	citedIDs := make([]string, 0, len(cited))
	for _, k := range cited {
		sources = append(sources, chunks[k-1])
		citedIDs = append(citedIDs, chunks[k-1].ID)
	}
	run.ChunksCited = len(cited)
	run.UsedInWords = len(cited) == 0 && researchUsed(text, chunks, strings.Join(append(earlierUser(history, t.Content), t.Content), " "))
	run.ReplyWords = len(strings.Fields(text))

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
	recID, err := a.store.SaveRecommendation(ctx, t.SessionID, replyID, variant, allIDs, citedIDs)
	if err != nil {
		note("record", err)
	}
	if run.AnalyserOK {
		if err := a.store.SaveSignals(ctx, toSignals(an, t, userMsgID)); err != nil {
			note("record", err)
		}
	}
	if err := a.store.RecordMoves(ctx, t.SessionID, plan.Moves); err != nil {
		note("record", err)
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

// draft is one reply from the model. cut is true when the ceiling stopped it
// even after the retry, so its last sentence may be unfinished
type draft struct {
	text string
	cut  bool
}

// respond makes one reply call under the safety ceiling and adds its usage
// to the run. a reply the ceiling cuts is asked for once more with twice the
// room: a reply in its band never comes near the ceiling, so hitting it means
// a runaway or a long think, and either deserves a second chance before
// anything is trimmed. an empty reply is an error, not a reply
func (a *Agent) respond(ctx context.Context, system string, msgs []llm.Message, run *store.Run) (draft, error) {
	limit := replyCeiling(a.cfg.RespondEffort)
	res, err := a.generate(ctx, system, msgs, limit, run)
	if err == nil && res.StopReason == stopMaxTokens {
		t0 := time.Now()
		again, rerr := a.generate(ctx, system, msgs, limit*2, run)
		if run.StagesMS != nil {
			run.StagesMS["retry"] += time.Since(t0).Milliseconds()
		}
		if rerr == nil && strings.TrimSpace(again.Text) != "" {
			res = again // otherwise the cut reply stands, to be trimmed
		}
	}
	run.StopReason = res.StopReason
	text := strings.TrimSpace(res.Text)
	if err == nil && text == "" {
		err = fmt.Errorf("empty reply (stop_reason=%s)", res.StopReason)
	}
	return draft{text: text, cut: res.StopReason == stopMaxTokens}, err
}

func (a *Agent) generate(ctx context.Context, system string, msgs []llm.Message, limit int, run *store.Run) (llm.Response, error) {
	res, err := a.model.Generate(ctx, llm.Request{
		System:    system,
		Messages:  msgs,
		MaxTokens: limit,
		Effort:    a.cfg.RespondEffort,
	})
	run.InputTokens += res.Usage.InputTokens
	run.OutputTokens += res.Usage.OutputTokens
	return res, err
}

// lintDraft is Lint plus what only the call knows: a draft the ceiling cut
// is truncated even when it happens to stop on a full stop
func lintDraft(d draft, sh Shape, prev []string) []string {
	v := Lint(d.text, sh, prev)
	if d.cut && !slices.Contains(v, "truncated") {
		v = append(v, "truncated")
	}
	return v
}

// variant returns the session's voice, choosing one by thompson sampling on
// its first turn. any failure falls back to the base voice, never an error
func (a *Agent) variant(ctx context.Context, sessionID string) string {
	if len(a.cfg.Variants) == 1 {
		return a.cfg.Variants[0]
	}
	if v, err := a.store.SessionVariant(ctx, sessionID); err == nil && v != "" {
		return resolveVariant(v) // a session from before a rename keeps working
	}
	stats, err := a.store.VariantStats(ctx)
	if err != nil {
		stats = nil // explore uniformly
	}
	v, err := a.store.ClaimSessionVariant(ctx, sessionID, PickVariant(stats, a.cfg.Variants, minRatingsEach))
	if err != nil {
		return DefaultVariant
	}
	return resolveVariant(v)
}

// progress is nil-safe: a turn without a listener just runs
func progress(t Turn, status string) {
	if t.Progress != nil {
		t.Progress(status)
	}
}

// assistantReplies are the earlier replies in the window, oldest first
func assistantReplies(history []store.Message) []string {
	var out []string
	for _, m := range history {
		if m.Role == "assistant" {
			out = append(out, m.Content)
		}
	}
	return out
}

// earlierUser are the person's messages in the window before this one,
// oldest first. the handler has already stored this turn, so it is dropped
// from the end when it is there
func earlierUser(history []store.Message, current string) []string {
	var out []string
	for _, m := range history {
		if m.Role == "user" {
			out = append(out, m.Content)
		}
	}
	if n := len(out); n > 0 && out[n-1] == current {
		out = out[:n-1]
	}
	return out
}

func lastOf(xs []string) string {
	if len(xs) == 0 {
		return ""
	}
	return xs[len(xs)-1]
}

// maxFacts bounds recall: four by similarity plus two open commitments is
// what one call returns at most, and a second recall must not widen that
const maxFacts = 6

// mergeFacts adds the facts in more that are not already in facts, up to n
func mergeFacts(facts, more []store.Fact, n int) []store.Fact {
	seen := map[string]bool{}
	for _, f := range facts {
		seen[f.ID] = true
	}
	for _, f := range more {
		if len(facts) >= n {
			break
		}
		if !seen[f.ID] {
			seen[f.ID] = true
			facts = append(facts, f)
		}
	}
	return facts
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

// retrieve searches the research library. vec is the message's vector when
// the query is the raw message, so the embedder is called once, not twice
func (a *Agent) retrieve(ctx context.Context, query string, vec []float32) ([]store.Chunk, error) {
	if len(vec) == 0 {
		vecs, err := a.embedder.Embed(ctx, []string{query})
		if err != nil {
			return nil, err
		}
		if len(vecs) == 0 || len(vecs[0]) == 0 {
			return nil, errors.New("embedder returned no vector")
		}
		vec = vecs[0]
	}
	return a.store.SearchChunks(ctx, vec, a.cfg.TopK, a.cfg.MinSimilarity)
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
