package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ pool *pgxpool.Pool }

func New(ctx context.Context, dsn string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close()              { s.pool.Close() }
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// models
//
// User is deliberately small. Email is here because the owner needs to see it
// on their own settings page; it must never be copied into any type that is
// serialised for another user (see Peer in social.go, which has no email).
type User struct {
	ID    string
	Email string
	// ConsentedAt is when the person last accepted the participation terms.
	// Nil until they do. Compare it with the date the terms last changed.
	ConsentedAt *time.Time

	// Handle is the anonymous name other users see. Nil until onboarding.
	Handle         *string
	PeerOptIn      bool
	NotifyReleases bool
}

// userCols is the one column list every user query scans, so adding a field
// means changing one string and one Scan call rather than hunting for queries.
const userCols = "id, email, consent_given_at, handle, peer_opt_in, notify_releases"

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.ConsentedAt, &u.Handle, &u.PeerOptIn, &u.NotifyReleases)
	return u, err
}

// ConsentedSince reports whether the person accepted the terms on or after
// termsUpdated. Agreeing before the terms last changed is not agreeing to
// what changed, so it does not count.
func (u User) ConsentedSince(termsUpdated time.Time) bool {
	return u.ConsentedAt != nil && !u.ConsentedAt.Before(termsUpdated)
}

// HasHandle reports whether onboarding is complete. Chat is gated on it: the
// agent asks what to call you before it asks anything else.
func (u User) HasHandle() bool { return u.Handle != nil && *u.Handle != "" }

// Message and Chunk cross the API boundary, so they carry explicit JSON tags.
// Without them Go marshals the field names verbatim and the frontend ends up
// reading `ID`, `URL`, `CreatedAt` — an API contract that is an accident of Go
// naming rather than a decision. Tag anything that gets serialised.
type Message struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

type Chunk struct {
	ID string `json:"id"`
	// Content is the retrieved passage. It is deliberately NOT serialised to
	// the client: it is prompt input, not user-facing text, and shipping it
	// would put a wall of abstract into every reply payload.
	Content string  `json:"-"`
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Year    *int    `json:"year"`
	Score   float64 `json:"score"`
	// Similarity is the raw cosine similarity before feedback weighting. The
	// agent uses it to refuse passages that are only vaguely related.
	Similarity float64 `json:"-"`
}

// ErrNotFound means the row does not exist or does not belong to the caller
// The two are deliberately indistinguishable to the client
var ErrNotFound = errors.New("not found")

// users
// GrantConsent records that the person accepted the terms now, and returns
// the exact time stored, so the caller reports what the database holds.
func (s *Store) GrantConsent(ctx context.Context, userID string) (time.Time, error) {
	var at time.Time
	err := s.pool.QueryRow(ctx,
		`UPDATE users SET consent_given_at = now() WHERE id = $1 RETURNING consent_given_at`,
		userID).Scan(&at)
	return at, err
}

// delete user to honor "delete my data" requuests
func (s *Store) DeleteUser(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
	return err
}

// sessions
func (s *Store) CreateSession(ctx context.Context, userID string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO sessions (user_id) VALUES ($1) RETURNING id`, userID).Scan(&id)
	return id, err
}

func (s *Store) SessionBelongsTo(ctx context.Context, sessionID, userID string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM sessions WHERE id=$1 AND user_id=$2)`,
		sessionID, userID).Scan(&ok)
	return ok, err
}

func (s *Store) TouchSession(ctx context.Context, sessionID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE sessions SET last_activity_at=now() WHERE id=$1`, sessionID)
	return err
}

// sidebar powering of the sessions
func (s *Store) ListSessions(ctx context.Context, userID string) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT s.id, COALESCE(s.title,'New conversation'), s.last_activity_at
		FROM sessions s WHERE s.user_id=$1
		ORDER BY s.last_activity_at DESC LIMIT 50`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []map[string]any{}
	for rows.Next() {
		var id, title string
		var ts time.Time
		if err := rows.Scan(&id, &title, &ts); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "title": title, "lastActivityAt": ts})
	}
	return out, rows.Err()
}

func (s *Store) SetSessionTitleIfEmpty(ctx context.Context, sessionID, title string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE sessions SET title=$2 WHERE id=$1 AND title IS NULL`, sessionID, title)
	return err
}

// messages
func (s *Store) AddMessage(ctx context.Context, sessionID, role, content string, flagged bool, score float64) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO messages (session_id, role, content, crisis_flagged, safety_score)
		VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		sessionID, role, content, flagged, score).Scan(&id)
	return id, err
}

// return messages in chronological order
func (s *Store) RecentMessages(ctx context.Context, sessionID string, n int) ([]Message, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, role::text, content, created_at FROM (
			SELECT id, role, content, created_at FROM messages
			WHERE session_id=$1 AND crisis_flagged=false
			ORDER BY created_at DESC LIMIT $2
		) t ORDER BY created_at ASC`, sessionID, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) CountMessages(ctx context.Context, sessionID string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM messages WHERE session_id=$1`, sessionID).Scan(&n)
	return n, err
}

// retrieve chunks for RAG
// SearchChunks does weighted vector search in two steps
// the inner query orders by raw distance, which is the only ordering the HNSW
// index can serve, and takes a generous candidate pool. the outer query drops
// anything below minSim and re-ranks the rest by similarity times
// helpful_weight. ordering by the weighted score directly (as the first
// version did) forces a full scan of every chunk on every message
func (s *Store) SearchChunks(ctx context.Context, vec []float32, k int, minSim float64) ([]Chunk, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, content, title, url, year, similarity, similarity * helpful_weight AS score
		FROM (
			SELECT c.id, c.content, d.title, COALESCE(d.url,'') AS url, d.year, c.helpful_weight,
			       1 - (c.embedding <=> $1::vector) AS similarity
			FROM research_chunks c
			JOIN research_docs d ON d.id = c.doc_id
			ORDER BY c.embedding <=> $1::vector
			LIMIT $2
		) nearest
		WHERE similarity >= $3
		ORDER BY score DESC
		LIMIT $4`, Vector(vec), k*4, minSim, k)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Chunk
	for rows.Next() {
		var c Chunk
		if err := rows.Scan(&c.ID, &c.Content, &c.Title, &c.URL, &c.Year, &c.Similarity, &c.Score); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) IncrementRetrieved(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE research_chunks SET times_retrieved = times_retrieved + 1 WHERE id = ANY($1)`, ids)
	return err
}

// LatestSummary is the episodic memory the agent carries between sessions.
func (s *Store) LatestSummary(ctx context.Context, userID string) (string, error) {
	var sum string
	err := s.pool.QueryRow(ctx, `
		SELECT summary FROM session_summaries
		WHERE user_id=$1 ORDER BY created_at DESC LIMIT 1`, userID).Scan(&sum)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return sum, err
}

func (s *Store) SaveSummary(ctx context.Context, sessionID, userID, summary string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO session_summaries (session_id, user_id, summary)
		VALUES ($1,$2,$3)
		ON CONFLICT (session_id) DO UPDATE SET summary=EXCLUDED.summary`,
		sessionID, userID, summary)
	return err
}

func (s *Store) SessionUser(ctx context.Context, sessionID string) (string, error) {
	var uid string
	err := s.pool.QueryRow(ctx, `SELECT user_id FROM sessions WHERE id=$1`, sessionID).Scan(&uid)
	return uid, err
}

// feedback
func (s *Store) SaveRecommendation(ctx context.Context, sessionID, messageID string, chunkIDs, citedIDs []string) (string, error) {
	if chunkIDs == nil {
		chunkIDs = []string{}
	}
	if citedIDs == nil {
		citedIDs = []string{}
	}
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO recommendations (session_id, message_id, source_chunk_ids, cited_chunk_ids)
		VALUES ($1,$2,$3,$4) RETURNING id`, sessionID, messageID, chunkIDs, citedIDs).Scan(&id)
	return id, err
}

// SaveFeedback records a rating and nudges the weight of the passages behind the reply
//   - ownership: the recommendation must belong to one of the caller's own
//     sessions, otherwise anyone holding a uuid could steer retrieval
//   - delta, not total. changing a thumbs up to a thumbs down moves the weight
//     by the difference, and clicking the same thumb twice moves it by zero
//     the first version added the full amount on every click
//   - credit goes to cited passages when there are any. retrieved but uncited
//     passages did not shape the reply and should not be judged by it
func (s *Store) SaveFeedback(ctx context.Context, recID, userID string, rating int, comment string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// FOR UPDATE serialises two quick clicks on the same reply, so both read
	// the correct previous rating instead of racing
	var prev int
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(f.rating, 0)
		FROM recommendations r
		JOIN sessions s ON s.id = r.session_id AND s.user_id = $2
		LEFT JOIN feedback f ON f.recommendation_id = r.id AND f.user_id = $2
		WHERE r.id = $1
		FOR UPDATE OF r`, recID, userID).Scan(&prev)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO feedback (recommendation_id, user_id, rating, comment)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (recommendation_id, user_id)
		DO UPDATE SET rating=EXCLUDED.rating, comment=EXCLUDED.comment`,
		recID, userID, rating, comment); err != nil {
		return err
	}

	// Small learning rate, hard clamp. Bounded updates keep one grumpy user
	// from burying a good source, and one enthusiast from dominating retrieval.
	const lr = 0.05
	if delta := rating - prev; delta != 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE research_chunks
			SET helpful_weight = LEAST(1.5, GREATEST(0.5, helpful_weight + $2))
			WHERE id = ANY(
				SELECT unnest(CASE WHEN cardinality(cited_chunk_ids) > 0
				                   THEN cited_chunk_ids ELSE source_chunk_ids END)
				FROM recommendations WHERE id = $1
			)`, recID, float64(delta)*lr); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// safety
func (s *Store) FlagCrisis(ctx context.Context, messageID, rule string, score float64) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO crisis_flags (message_id, matched_rule, classifier_score)
		VALUES ($1,$2,$3)`, messageID, rule, score)
	return err
}

// helpers
// Vectore formats a Go slice as the literal pgvectore expects: [0.1,0.2...]
// pgx has no native vector type, so we hand it a string and cast in SQL.
func Vector(v []float32) string {
	parts := make([]string, len(v))
	for i, f := range v {
		parts[i] = fmt.Sprintf("%g", f)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
