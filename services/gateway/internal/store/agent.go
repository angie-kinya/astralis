package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Signals is one structured observation of one user message. Pointers are
// nullable columns: nil means the message carried no evidence either way
type Signals struct {
	MessageID       string
	UserID          string
	SessionID       string
	Intent          string
	Exhaustion      *int16
	Cynicism        *int16
	ReducedEfficacy *int16
	Demands         []string
	Resources       []string
	Valence         *float64
	AnalyserVersion string
}

func (s *Store) SaveSignals(ctx context.Context, sig Signals) error {
	if sig.Demands == nil {
		sig.Demands = []string{}
	}
	if sig.Resources == nil {
		sig.Resources = []string{}
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO turn_signals (message_id, user_id, session_id, intent, exhaustion,
			cynicism, reduced_efficacy, demands, resources, valence, analyser_version)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (message_id) DO NOTHING`,
		sig.MessageID, sig.UserID, sig.SessionID, sig.Intent, sig.Exhaustion,
		sig.Cynicism, sig.ReducedEfficacy, sig.Demands, sig.Resources, sig.Valence,
		sig.AnalyserVersion)
	return err
}

// Run is the audit record of one chat turn
type Run struct {
	UserID         string
	SessionID      string
	UserMessageID  string
	ReplyMessageID string
	Route          string
	Intent         string
	SafetyRule     string
	AnalyserOK     bool
	RetrievalUsed  bool
	ChunksFound    int
	ChunksCited    int
	TopSimilarity  *float64
	InputTokens    int
	OutputTokens   int
	StopReason     string
	StagesMS       map[string]int64
	TotalMS        int64
	Error          string

	// v2: the voice, the shape it was asked for and whether it held
	PromptVariant   string
	WordBudget      int
	ReplyWords      int
	StyleViolations []string // first-pass lint, kept even when the repair fixed them
	Repaired        bool     // the revised draft is the one that shipped
	FactsUsed       int
	Moves           []string

	// v2.2: the turn plan and what became of it
	Stance      string
	BandMin     int
	BandMax     int
	Advice      string // none | offer | allow | give
	CiteMode    string // none | words | markers
	Truncated   bool   // the ceiling cut the shipped reply even after the retry, so it lost a sentence
	UsedInWords bool   // the reply drew on the excerpts without markers (a heuristic)
	Topic       string
	TopicTurns  int
}

func (s *Store) SaveRun(ctx context.Context, r Run) error {
	stages, err := json.Marshal(r.StagesMS)
	if err != nil {
		return err
	}
	if r.StyleViolations == nil {
		r.StyleViolations = []string{}
	}
	if r.Moves == nil {
		r.Moves = []string{}
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO agent_runs (user_id, session_id, user_message_id, reply_message_id, route,
			intent, safety_rule, analyser_ok, retrieval_used, chunks_found, chunks_cited,
			top_similarity, input_tokens, output_tokens, stop_reason, stages_ms, total_ms, error,
			prompt_variant, word_budget, reply_words, style_violations, repaired, facts_used, moves,
			stance, band_min, band_max, advice, cite_mode, truncated, used_in_words, topic, topic_turns)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16::jsonb,$17,$18,
			$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32,$33,$34)`,
		r.UserID, r.SessionID, nullIfEmpty(r.UserMessageID), nullIfEmpty(r.ReplyMessageID), r.Route,
		nullIfEmpty(r.Intent), nullIfEmpty(r.SafetyRule), r.AnalyserOK, r.RetrievalUsed,
		r.ChunksFound, r.ChunksCited, r.TopSimilarity, r.InputTokens, r.OutputTokens,
		nullIfEmpty(r.StopReason), string(stages), r.TotalMS, nullIfEmpty(r.Error),
		nullIfEmpty(r.PromptVariant), nullIfZero(r.WordBudget), nullIfZero(r.ReplyWords),
		r.StyleViolations, r.Repaired, r.FactsUsed, r.Moves,
		nullIfEmpty(r.Stance), nullIfZero(r.BandMin), nullIfZero(r.BandMax), nullIfEmpty(r.Advice),
		nullIfEmpty(r.CiteMode), r.Truncated, r.UsedInWords, nullIfEmpty(r.Topic), r.TopicTurns)
	return err
}

// TopicState is the thread a session is on: the analyser's latest topic and
// how many turns in a row have been about it
type TopicState struct {
	Topic string
	Turns int
}

// LastTopic is the active topic from the session's latest answered turn, or
// the zero state on a first turn. it reads agent_runs rather than a table of
// its own: the run already records every plan, and crisis and failed turns
// carry no topic, so they never move a thread
func (s *Store) LastTopic(ctx context.Context, sessionID string) (TopicState, error) {
	var t TopicState
	var topic *string
	err := s.pool.QueryRow(ctx, `
		SELECT topic, topic_turns FROM agent_runs
		WHERE session_id = $1 AND route = 'agent'
		ORDER BY created_at DESC LIMIT 1`, sessionID).Scan(&topic, &t.Turns)
	if errors.Is(err, pgx.ErrNoRows) {
		return TopicState{}, nil
	}
	if topic != nil {
		t.Topic = *topic
	}
	return t, err
}

// RecentSummaries returns the last n session summaries, newest first
// one summary only ever remembered the previous conversation; three give the agent a sense of how things have moved
func (s *Store) RecentSummaries(ctx context.Context, userID string, n int) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT summary FROM session_summaries
		WHERE user_id=$1 ORDER BY created_at DESC LIMIT $2`, userID, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var sum string
		if err := rows.Scan(&sum); err != nil {
			return nil, err
		}
		out = append(out, sum)
	}
	return out, rows.Err()
}

// admin

// Overview is the health of the agent over a time window, computed from agent_runs alone.
// every field is an aggregate so no user is identifiable
type Overview struct {
	WindowHours      int      `json:"windowHours"`
	Turns            int      `json:"turns"`
	CrisisTurns      int      `json:"crisisTurns"`
	FailedTurns      int      `json:"failedTurns"`
	AnalyserFailures int      `json:"analyserFailures"`
	ClassifierDown   int      `json:"classifierDown"`
	RetrievalRate    float64  `json:"retrievalRate"`
	CitationRate     float64  `json:"citationRate"`
	TruncatedReplies int      `json:"truncatedReplies"`
	P50MS            *float64 `json:"p50Ms"`
	P95MS            *float64 `json:"p95Ms"`
	InputTokens      int64    `json:"inputTokens"`
	OutputTokens     int64    `json:"outputTokens"`
	AvgTopSimilarity *float64 `json:"avgTopSimilarity"`
	ActiveUsers      int      `json:"activeUsers"`
	// v2 voice health: how often the first draft broke a style rule, how
	// often the repair shipped, and how often recall had something to offer
	StyleViolationRate float64 `json:"styleViolationRate"`
	RepairedRate       float64 `json:"repairedRate"`
	FactsUsedRate      float64 `json:"factsUsedRate"`
	// v2.2: the share of answered turns whose reply lost a sentence to the
	// ceiling (the target is zero), and how turns split across stances
	TruncationRate float64        `json:"truncationRate"`
	StanceMix      map[string]int `json:"stanceMix"`
	LastError      *string        `json:"lastError"`
	LastErrorAt    *string        `json:"lastErrorAt"`
}

func (s *Store) AgentOverview(ctx context.Context, window time.Duration) (Overview, error) {
	o := Overview{WindowHours: int(window.Hours()), StanceMix: map[string]int{}}
	err := s.pool.QueryRow(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE route='crisis'),
		       count(*) FILTER (WHERE route='failed'),
		       count(*) FILTER (WHERE route<>'crisis' AND NOT analyser_ok),
		       count(*) FILTER (WHERE safety_rule='classifier_unavailable'),
		       COALESCE(avg(retrieval_used::int) FILTER (WHERE route='agent'), 0)::float8,
		       COALESCE(avg((chunks_cited > 0)::int) FILTER (WHERE retrieval_used), 0)::float8,
		       count(*) FILTER (WHERE stop_reason='max_tokens'),
		       percentile_cont(0.5)  WITHIN GROUP (ORDER BY total_ms) FILTER (WHERE route='agent'),
		       percentile_cont(0.95) WITHIN GROUP (ORDER BY total_ms) FILTER (WHERE route='agent'),
		       COALESCE(sum(input_tokens), 0), COALESCE(sum(output_tokens), 0),
		       avg(top_similarity)::float8,
		       count(DISTINCT user_id),
		       COALESCE(avg((cardinality(style_violations) > 0)::int) FILTER (WHERE route='agent'), 0)::float8,
		       COALESCE(avg(repaired::int) FILTER (WHERE route='agent'), 0)::float8,
		       COALESCE(avg((facts_used > 0)::int) FILTER (WHERE route='agent'), 0)::float8,
		       COALESCE(avg(truncated::int) FILTER (WHERE route='agent'), 0)::float8
		FROM agent_runs WHERE created_at > now() - $1::interval`, window).Scan(
		&o.Turns, &o.CrisisTurns, &o.FailedTurns, &o.AnalyserFailures, &o.ClassifierDown,
		&o.RetrievalRate, &o.CitationRate, &o.TruncatedReplies, &o.P50MS, &o.P95MS,
		&o.InputTokens, &o.OutputTokens, &o.AvgTopSimilarity, &o.ActiveUsers,
		&o.StyleViolationRate, &o.RepairedRate, &o.FactsUsedRate, &o.TruncationRate)
	if err != nil {
		return o, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT stance, count(*)::int FROM agent_runs
		WHERE created_at > now() - $1::interval AND route = 'agent' AND stance IS NOT NULL
		GROUP BY stance`, window)
	if err != nil {
		return o, err
	}
	defer rows.Close()
	for rows.Next() {
		var st string
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			return o, err
		}
		o.StanceMix[st] = n
	}
	if err := rows.Err(); err != nil {
		return o, err
	}
	// the most recent error of any kind
	var at time.Time
	var msg string
	if err := s.pool.QueryRow(ctx, `
		SELECT error, created_at FROM agent_runs
		WHERE error IS NOT NULL ORDER BY created_at DESC LIMIT 1`).Scan(&msg, &at); err == nil {
		ts := at.Format(time.RFC3339)
		o.LastError, o.LastErrorAt = &msg, &ts
	}
	return o, nil
}

// RunRow is one agent_runs row as the admin page shows it
type RunRow struct {
	ID            string           `json:"id"`
	Route         string           `json:"route"`
	Intent        *string          `json:"intent"`
	SafetyRule    *string          `json:"safetyRule"`
	AnalyserOK    bool             `json:"analyserOk"`
	RetrievalUsed bool             `json:"retrievalUsed"`
	ChunksFound   int              `json:"chunksFound"`
	ChunksCited   int              `json:"chunksCited"`
	TopSimilarity *float64         `json:"topSimilarity"`
	InputTokens   int              `json:"inputTokens"`
	OutputTokens  int              `json:"outputTokens"`
	StopReason    *string          `json:"stopReason"`
	StagesMS      map[string]int64 `json:"stagesMs"`
	TotalMS       int              `json:"totalMs"`
	Error         *string          `json:"error"`
	CreatedAt     time.Time        `json:"createdAt"`

	PromptVariant   *string  `json:"promptVariant"`
	WordBudget      *int     `json:"wordBudget"`
	ReplyWords      *int     `json:"replyWords"`
	StyleViolations []string `json:"styleViolations"`
	Repaired        bool     `json:"repaired"`
	FactsUsed       int      `json:"factsUsed"`
	Moves           []string `json:"moves"`

	// the topic itself stays out: it is the person's words, and this page
	// is for the agent's behaviour
	Stance      *string `json:"stance"`
	BandMin     *int    `json:"bandMin"`
	BandMax     *int    `json:"bandMax"`
	Advice      *string `json:"advice"`
	CiteMode    *string `json:"citeMode"`
	Truncated   bool    `json:"truncated"`
	UsedInWords bool    `json:"usedInWords"`
	TopicTurns  int     `json:"topicTurns"`
}

func (s *Store) RecentRuns(ctx context.Context, limit int) ([]RunRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, route, intent, safety_rule, analyser_ok, retrieval_used, chunks_found,
		       chunks_cited, top_similarity, input_tokens, output_tokens, stop_reason,
		       stages_ms, total_ms, error, created_at,
		       prompt_variant, word_budget, reply_words, style_violations, repaired, facts_used, moves,
		       stance, band_min, band_max, advice, cite_mode, truncated, used_in_words, topic_turns
		FROM agent_runs ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []RunRow{}
	for rows.Next() {
		var r RunRow
		var stages []byte
		if err := rows.Scan(&r.ID, &r.Route, &r.Intent, &r.SafetyRule, &r.AnalyserOK,
			&r.RetrievalUsed, &r.ChunksFound, &r.ChunksCited, &r.TopSimilarity,
			&r.InputTokens, &r.OutputTokens, &r.StopReason, &stages, &r.TotalMS,
			&r.Error, &r.CreatedAt, &r.PromptVariant, &r.WordBudget, &r.ReplyWords,
			&r.StyleViolations, &r.Repaired, &r.FactsUsed, &r.Moves,
			&r.Stance, &r.BandMin, &r.BandMax, &r.Advice, &r.CiteMode, &r.Truncated,
			&r.UsedInWords, &r.TopicTurns); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(stages, &r.StagesMS)
		out = append(out, r)
	}
	return out, rows.Err()
}

// v2: trajectory, noticing moves, variants

// Pattern is the trajectory of the last 21 days of turn_signals. avg ignores
// NULLs, which is the not_evident semantics 0004 designed: a message that
// showed nothing either way does not pull the average towards low
type Pattern struct {
	ExhRecent       float64 // mean exhaustion, last 7 days (0 when none)
	ExhBefore       float64 // mean exhaustion, 8 to 21 days ago (0 when none)
	NRecent         int     // rated messages in the last 7 days
	RecurringDemand string  // a demand raised in 3+ distinct sessions, or ""
}

// crisis turns never reach turn_signals (the analyser never sees them), so
// they can never shape a trajectory
func (s *Store) Pattern(ctx context.Context, userID string) (Pattern, error) {
	var p Pattern
	var recent, before *float64
	var demand *string
	err := s.pool.QueryRow(ctx, `
		WITH w AS (
			SELECT * FROM turn_signals
			WHERE user_id = $1 AND created_at > now() - interval '21 days'
		)
		SELECT
			avg(exhaustion) FILTER (WHERE created_at >  now() - interval '7 days')::float8,
			avg(exhaustion) FILTER (WHERE created_at <= now() - interval '7 days')::float8,
			count(exhaustion) FILTER (WHERE created_at > now() - interval '7 days')::int,
			(SELECT d FROM w, unnest(w.demands) d GROUP BY d
			   HAVING count(DISTINCT w.session_id) >= 3
			   ORDER BY count(DISTINCT w.session_id) DESC, d LIMIT 1)
		FROM w`, userID).Scan(&recent, &before, &p.NRecent, &demand)
	if recent != nil {
		p.ExhRecent = *recent
	}
	if before != nil {
		p.ExhBefore = *before
	}
	if demand != nil {
		p.RecurringDemand = *demand
	}
	return p, err
}

func (s *Store) SessionMoves(ctx context.Context, sessionID string) (map[string]bool, error) {
	rows, err := s.pool.Query(ctx, `SELECT move FROM session_moves WHERE session_id = $1`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			return nil, err
		}
		out[m] = true
	}
	return out, rows.Err()
}

func (s *Store) RecordMoves(ctx context.Context, sessionID string, moves []string) error {
	if len(moves) == 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO session_moves (session_id, move)
		SELECT $1, unnest($2::text[]) ON CONFLICT DO NOTHING`, sessionID, moves)
	return err
}

// SessionVariant is the voice already chosen for a session, or ""
func (s *Store) SessionVariant(ctx context.Context, sessionID string) (string, error) {
	var v *string
	err := s.pool.QueryRow(ctx, `SELECT prompt_variant FROM sessions WHERE id = $1`, sessionID).Scan(&v)
	if err != nil || v == nil {
		return "", err
	}
	return *v, nil
}

// ClaimSessionVariant sets the voice only if none is set and returns whichever
// won, so two quick first messages in one session cannot pick two voices
func (s *Store) ClaimSessionVariant(ctx context.Context, sessionID, variant string) (string, error) {
	var v string
	err := s.pool.QueryRow(ctx, `
		UPDATE sessions SET prompt_variant = COALESCE(prompt_variant, $2)
		WHERE id = $1 RETURNING prompt_variant`, sessionID, variant).Scan(&v)
	return v, err
}

// VariantStats is {helpful, unhelpful} per voice: thumbs on replies plus the
// end-of-session card. a neutral rating (0) credits neither side
func (s *Store) VariantStats(ctx context.Context) (map[string][2]int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT v, sum(up)::int, sum(down)::int FROM (
			SELECT r.prompt_variant AS v, (f.rating > 0)::int AS up, (f.rating < 0)::int AS down
			FROM feedback f JOIN recommendations r ON r.id = f.recommendation_id
			WHERE r.prompt_variant IS NOT NULL
			UNION ALL
			SELECT prompt_variant, worth_it::int, (NOT worth_it)::int
			FROM sessions WHERE prompt_variant IS NOT NULL AND worth_it IS NOT NULL
		) t GROUP BY v`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][2]int{}
	for rows.Next() {
		var v string
		var up, down int
		if err := rows.Scan(&v, &up, &down); err != nil {
			return nil, err
		}
		out[v] = [2]int{up, down}
	}
	return out, rows.Err()
}

func nullIfZero(v int) any {
	if v == 0 {
		return nil
	}
	return v
}

// nullIfEmpty turns "" into SQL NULL, so optional text and uuid columns stay
// NULL rather than holding an empty string (which a uuid column rejects)
func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}
