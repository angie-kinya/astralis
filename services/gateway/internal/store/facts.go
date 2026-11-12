package store

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Fact is one durable thing Astralis knows about a person: someone in their
// work life, an event, a commitment, something that helped, or context.
// knowledge lives in rows, not prose, so it can be recalled by relevance,
// dated, listed in settings and deleted one at a time
type Fact struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"` // person|event|commitment|helped|context
	Subject    string    `json:"subject"`
	Detail     string    `json:"detail"`
	Status     string    `json:"status"` // open|done|dropped
	Mentions   int       `json:"mentions"`
	FirstSeen  time.Time `json:"firstSeen"`
	LastSeen   time.Time `json:"lastSeen"`
	Similarity float64   `json:"-"`
}

const factCols = "id, kind, subject, detail, status, mentions, first_seen, last_seen"

func scanFacts(rows pgx.Rows, withSim bool) ([]Fact, error) {
	defer rows.Close()
	out := []Fact{} // empty, not nil: see RecentMessages
	for rows.Next() {
		var f Fact
		dst := []any{&f.ID, &f.Kind, &f.Subject, &f.Detail, &f.Status, &f.Mentions, &f.FirstSeen, &f.LastSeen}
		if withSim {
			dst = append(dst, &f.Similarity)
		}
		if err := rows.Scan(dst...); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// UpsertFact writes one extracted fact. the unique index on lower(subject)
// is the entity resolution: a repeat raises mentions instead of duplicating
func (s *Store) UpsertFact(ctx context.Context, userID, sessionID string, f Fact, vec []float32) error {
	if f.Status == "" {
		f.Status = "open"
	}
	var emb any
	if len(vec) > 0 {
		emb = Vector(vec)
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO person_facts (user_id, kind, subject, detail, status, embedding, source_session_id)
		VALUES ($1,$2,$3,$4,$5,$6::vector,$7)
		ON CONFLICT (user_id, kind, lower(subject)) DO UPDATE
		   SET detail = EXCLUDED.detail, status = EXCLUDED.status,
		       embedding = COALESCE(EXCLUDED.embedding, person_facts.embedding),
		       source_session_id = COALESCE(EXCLUDED.source_session_id, person_facts.source_session_id),
		       mentions = person_facts.mentions + 1, last_seen = now()`,
		userID, f.Kind, strings.TrimSpace(f.Subject), strings.TrimSpace(f.Detail), f.Status, emb, nullIfEmpty(sessionID))
	return err
}

// RecallFacts returns the k facts nearest to the message above minSim, plus up
// to 2 open commitments from the last 21 days whatever their similarity: a
// commitment is worth asking about even when today's message is about something else
func (s *Store) RecallFacts(ctx context.Context, userID string, vec []float32, k int, minSim float64) ([]Fact, error) {
	rows, err := s.pool.Query(ctx, `
		WITH near AS (
			SELECT `+factCols+`, 1 - (embedding <=> $2::vector) AS sim
			FROM person_facts
			WHERE user_id = $1 AND embedding IS NOT NULL
			ORDER BY embedding <=> $2::vector
			LIMIT $3
		), commits AS (
			SELECT `+factCols+`, 0::float8 AS sim
			FROM person_facts
			WHERE user_id = $1 AND kind = 'commitment' AND status = 'open'
			  AND last_seen > now() - interval '21 days'
			  AND id NOT IN (SELECT id FROM near WHERE sim >= $4)
			ORDER BY last_seen DESC
			LIMIT 2
		)
		SELECT * FROM near WHERE sim >= $4
		UNION ALL
		SELECT * FROM commits
		ORDER BY last_seen DESC`, userID, Vector(vec), k, minSim)
	if err != nil {
		return nil, err
	}
	return scanFacts(rows, true)
}

// ListFacts is the "what Astralis remembers" page, newest first
func (s *Store) ListFacts(ctx context.Context, userID string) ([]Fact, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+factCols+` FROM person_facts
		WHERE user_id = $1 ORDER BY last_seen DESC LIMIT 200`, userID)
	if err != nil {
		return nil, err
	}
	return scanFacts(rows, false)
}

// KnownSubjects feeds the extractor, so a repeat reuses the exact subject
func (s *Store) KnownSubjects(ctx context.Context, userID string) ([]Fact, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+factCols+` FROM person_facts
		WHERE user_id = $1 ORDER BY mentions DESC, last_seen DESC LIMIT 40`, userID)
	if err != nil {
		return nil, err
	}
	return scanFacts(rows, false)
}

func (s *Store) DeleteFact(ctx context.Context, userID, factID string) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM person_facts WHERE id = $1 AND user_id = $2`, factID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// LastTime is the quiet line at the top of a returning session: the most
// recent open commitment or event from 21 days, else the most recent fact
func (s *Store) LastTime(ctx context.Context, userID string) (*Fact, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+factCols+` FROM person_facts
		WHERE user_id = $1 AND last_seen > now() - interval '21 days'
		ORDER BY (kind IN ('commitment','event') AND status = 'open') DESC, last_seen DESC
		LIMIT 1`, userID)
	if err != nil {
		return nil, err
	}
	fs, err := scanFacts(rows, false)
	if err != nil || len(fs) == 0 {
		return nil, err
	}
	return &fs[0], nil
}

// RateSession records the end-of-session card. a later answer replaces an earlier one
func (s *Store) RateSession(ctx context.Context, userID, sessionID string, worth bool) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE sessions SET worth_it = $3, rated_at = now()
		WHERE id = $1 AND user_id = $2`, sessionID, userID, worth)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
