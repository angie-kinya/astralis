package store

import (
	"context"
	"encoding/json"
	"time"
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
}

func (s *Store) SaveRun(ctx context.Context, r Run) error {
	stages, err := json.Marshal(r.StagesMS)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO agent_runs (user_id, session_id, user_message_id, reply_message_id, route,
			intent, safety_rule, analyser_ok, retrieval_used, chunks_found, chunks_cited,
			top_similarity, input_tokens, output_tokens, stop_reason, stages_ms, total_ms, error)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16::jsonb,$17,$18)`,
		r.UserID, r.SessionID, nullIfEmpty(r.UserMessageID), nullIfEmpty(r.ReplyMessageID), r.Route,
		nullIfEmpty(r.Intent), nullIfEmpty(r.SafetyRule), r.AnalyserOK, r.RetrievalUsed,
		r.ChunksFound, r.ChunksCited, r.TopSimilarity, r.InputTokens, r.OutputTokens,
		nullIfEmpty(r.StopReason), string(stages), r.TotalMS, nullIfEmpty(r.Error))
	return err
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
	LastError        *string  `json:"lastError"`
	LastErrorAt      *string  `json:"lastErrorAt"`
}

func (s *Store) AgentOverview(ctx context.Context, window time.Duration) (Overview, error) {
	o := Overview{WindowHours: int(window.Hours())}
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
		       count(DISTINCT user_id)
		FROM agent_runs WHERE created_at > now() - $1::interval`, window).Scan(
		&o.Turns, &o.CrisisTurns, &o.FailedTurns, &o.AnalyserFailures, &o.ClassifierDown,
		&o.RetrievalRate, &o.CitationRate, &o.TruncatedReplies, &o.P50MS, &o.P95MS,
		&o.InputTokens, &o.OutputTokens, &o.AvgTopSimilarity, &o.ActiveUsers)
	if err != nil {
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
}

func (s *Store) RecentRuns(ctx context.Context, limit int) ([]RunRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, route, intent, safety_rule, analyser_ok, retrieval_used, chunks_found,
		       chunks_cited, top_similarity, input_tokens, output_tokens, stop_reason,
		       stages_ms, total_ms, error, created_at
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
			&r.Error, &r.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(stages, &r.StagesMS)
		out = append(out, r)
	}
	return out, rows.Err()
}

// nullIfEmpty turns "" into SQL NULL, so optional text and uuid columns stay
// NULL rather than holding an empty string (which a uuid column rejects)
func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}
