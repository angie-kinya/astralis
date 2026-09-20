package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// PresenceWindow is how recently someone must have made a request to count as
// online. Presence is computed at read time from last_seen_at rather than
// stored as a flag, so a crashed tab cannot leave a person showing as online
// indefinitely — the fact expires on its own.
const PresenceWindow = 2 * time.Minute

// ------------------------------------------------------------------ identity

// Peer is another user as seen by a user. Note what is absent: no email, no
// GitHub display name, no id that means anything outside this database. A
// handle is the entire public identity.
type Peer struct {
	ID       string `json:"id"`
	Handle   string `json:"handle"`
	Online   bool   `json:"online"`
	LastSeen string `json:"lastSeen"`
	Unread   int    `json:"unread"`
}

// SetHandle claims an anonymous display name. The UNIQUE constraint does the
// work; a collision comes back as an error the handler turns into a 409 so the
// UI can ask for another one.
func (s *Store) SetHandle(ctx context.Context, userID, handle string) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET handle=$2 WHERE id=$1`, userID, handle)
	return err
}

func (s *Store) HandleTaken(ctx context.Context, handle string) (bool, error) {
	var taken bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE lower(handle)=lower($1))`, handle).Scan(&taken)
	return taken, err
}

func (s *Store) SetPeerOptIn(ctx context.Context, userID string, in bool) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET peer_opt_in=$2 WHERE id=$1`, userID, in)
	return err
}

// TouchPresence is called by middleware on every authenticated request. It is
// deliberately fire-and-forget at the call site: presence is nice to have and
// must never be able to fail a real request.
func (s *Store) TouchPresence(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET last_seen_at=now() WHERE id=$1`, userID)
	return err
}

// ------------------------------------------------------------------ peers

// ListPeers returns everyone who has opted in, with unread counts, online
// first. The LEFT JOIN counts unread messages from that peer in one query
// rather than N+1 round trips.
func (s *Store) ListPeers(ctx context.Context, userID string) ([]Peer, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.id, COALESCE(u.handle,'someone'), u.last_seen_at,
		       (u.last_seen_at > now() - $2::interval) AS online,
		       COUNT(m.id) FILTER (WHERE m.read_at IS NULL) AS unread
		FROM users u
		LEFT JOIN peer_messages m
		       ON m.sender_id = u.id AND m.recipient_id = $1 AND m.crisis_flagged = false
		WHERE u.peer_opt_in = true AND u.id <> $1 AND u.handle IS NOT NULL
		GROUP BY u.id, u.handle, u.last_seen_at
		ORDER BY online DESC, u.last_seen_at DESC
		LIMIT 100`, userID, PresenceWindow)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Peer{}
	for rows.Next() {
		var p Peer
		var seen time.Time
		if err := rows.Scan(&p.ID, &p.Handle, &seen, &p.Online, &p.Unread); err != nil {
			return nil, err
		}
		p.LastSeen = seen.Format(time.RFC3339)
		out = append(out, p)
	}
	return out, rows.Err()
}

type PeerMessage struct {
	ID        string    `json:"id"`
	Mine      bool      `json:"mine"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
	ReadAt    *string   `json:"readAt"`
}

// Conversation reads both directions of a pair. Flagged messages are excluded
// for the same reason they are excluded from agent history: they were never
// delivered and they do not belong in anyone's record of the conversation.
func (s *Store) Conversation(ctx context.Context, userID, peerID string, n int) ([]PeerMessage, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, sender_id = $1 AS mine, body, created_at, read_at FROM (
			SELECT id, sender_id, body, created_at, read_at
			FROM peer_messages
			WHERE crisis_flagged = false
			  AND ((sender_id=$1 AND recipient_id=$2) OR (sender_id=$2 AND recipient_id=$1))
			ORDER BY created_at DESC LIMIT $3
		) t ORDER BY created_at ASC`, userID, peerID, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []PeerMessage{}
	for rows.Next() {
		var m PeerMessage
		var read *time.Time
		if err := rows.Scan(&m.ID, &m.Mine, &m.Body, &m.CreatedAt, &read); err != nil {
			return nil, err
		}
		if read != nil {
			v := read.Format(time.RFC3339)
			m.ReadAt = &v
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// SendPeerMessage refuses to deliver to someone who has not opted in — checked
// here in the same statement rather than in the handler, so there is no window
// between the check and the write.
func (s *Store) SendPeerMessage(ctx context.Context, senderID, recipientID, body string, flagged bool) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO peer_messages (sender_id, recipient_id, body, crisis_flagged)
		SELECT $1, $2, $3, $4
		WHERE EXISTS (SELECT 1 FROM users WHERE id=$2 AND peer_opt_in=true)
		RETURNING id`, senderID, recipientID, body, flagged).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrPeerUnavailable
	}
	return id, err
}

var ErrPeerUnavailable = errors.New("peer is not accepting messages")

func (s *Store) MarkConversationRead(ctx context.Context, userID, peerID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE peer_messages SET read_at=now()
		WHERE recipient_id=$1 AND sender_id=$2 AND read_at IS NULL`, userID, peerID)
	return err
}

func (s *Store) UnreadTotal(ctx context.Context, userID string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM peer_messages
		WHERE recipient_id=$1 AND read_at IS NULL AND crisis_flagged=false`, userID).Scan(&n)
	return n, err
}

// ------------------------------------------------------------------ notes

type Note struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (s *Store) ListNotes(ctx context.Context, userID string) ([]Note, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, COALESCE(title,''), body, updated_at
		FROM notes WHERE user_id=$1 ORDER BY updated_at DESC LIMIT 200`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Note{}
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.Title, &n.Body, &n.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) CreateNote(ctx context.Context, userID string) (Note, error) {
	var n Note
	err := s.pool.QueryRow(ctx, `
		INSERT INTO notes (user_id) VALUES ($1)
		RETURNING id, COALESCE(title,''), body, updated_at`, userID).
		Scan(&n.ID, &n.Title, &n.Body, &n.UpdatedAt)
	return n, err
}

// SaveNote scopes the UPDATE by user_id as well as id. Without that second
// predicate, knowing a note's UUID would be enough to overwrite it.
func (s *Store) SaveNote(ctx context.Context, userID, noteID, title, body string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE notes SET title=NULLIF($3,''), body=$4, updated_at=now()
		WHERE id=$2 AND user_id=$1`, userID, noteID, title, body)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) DeleteNote(ctx context.Context, userID, noteID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM notes WHERE id=$2 AND user_id=$1`, userID, noteID)
	return err
}

// FlagPeerCrisis is the audit row for a peer message the filter stopped.
// Separate from FlagCrisis because the two point at different tables; the
// CHECK constraint in 0002 makes it impossible to set both.
func (s *Store) FlagPeerCrisis(ctx context.Context, peerMessageID, rule string, score float64) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO crisis_flags (peer_message_id, matched_rule, classifier_score)
		VALUES ($1,$2,$3)`, peerMessageID, rule, score)
	return err
}

// ------------------------------------------------------------------ batch summary

// PendingSummary identifies a conversation that has gone quiet without being
// summarised.
type PendingSummary struct {
	SessionID string
	UserID    string
}

// SessionsNeedingSummary finds conversations worth remembering that nothing
// has remembered yet.
//
// This exists so summarisation can run as a scheduled batch with no queue and
// no Redis at all — which is what makes a genuinely free deployment possible.
// The queue path (cmd/worker) is still the better design under load; this is
// the same work, derived from state rather than from an event.
//
// Deriving from state has one real advantage: a summary missed because the
// browser never fired `pagehide`, or because Redis was down, is picked up on
// the next run instead of being lost.
func (s *Store) SessionsNeedingSummary(ctx context.Context, idleFor time.Duration, limit int) ([]PendingSummary, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT s.id, s.user_id
		FROM sessions s
		LEFT JOIN session_summaries ss ON ss.session_id = s.id
		WHERE ss.id IS NULL
		  AND s.last_activity_at < now() - $1::interval
		  AND (SELECT count(*) FROM messages m
		       WHERE m.session_id = s.id AND m.crisis_flagged = false) >= 4
		ORDER BY s.last_activity_at ASC
		LIMIT $2`, idleFor, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []PendingSummary{}
	for rows.Next() {
		var p PendingSummary
		if err := rows.Scan(&p.SessionID, &p.UserID); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
