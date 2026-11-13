package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"astralis/gateway/internal/game"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// games of every kind. the rules live in the engines under internal/game and
// this file never reads a state: it stores opaque json, arbitrates races and
// keeps the columns (turn, winner, level) the queries below need. rows come
// back unshaped (both players, both handles) and the api turns each into the
// view of whoever is receiving it

var (
	ErrRateLimited   = errors.New("too many invites")
	ErrInvitePending = errors.New("an invite is already open between these two")
	ErrInviteGone    = errors.New("invite already handled or expired")
	// ErrStale means the version moved on, or the game ended, since the client read it
	ErrStale = errors.New("game changed since it was read")
)

const (
	invitesPerHour = 5
	// the pair index name from 0008; a violation on it means an open invite exists
	pendingPairIndex = "uq_game_invites_pending_pair"
	// the level 0009 gives every row from before levels: the 4x4 board
	legacyLevel = 2
)

// dbOptions is how options are written to jsonb. topic and mode are always
// present, if empty: the daily unique index (0009) keys on options->>'topic'
// and options->>'mode', and a missing key is NULL there, which never collides,
// so a sudoku daily could be dealt twice
type dbOptions struct {
	Level int    `json:"level,omitempty"`
	Topic string `json:"topic"`
	Mode  string `json:"mode"`
	Day   string `json:"day,omitempty"`
}

func optionsJSON(o game.Options) string {
	b, _ := json.Marshal(dbOptions(o)) // plain strings and an int cannot fail
	return string(b)
}

func scanOptions(raw []byte, o *game.Options) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, o)
}

// querier is the part of a pool or a transaction these helpers need
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// ------------------------------------------------------------------ invites

type InviteRow struct {
	ID         string
	FromID     string
	FromHandle string
	ToID       string
	ToHandle   string
	Status     string
	GameID     *string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	Kind       string
	Options    game.Options
}

const inviteSelect = `
	SELECT i.id, i.from_id, COALESCE(f.handle,'someone'), i.to_id, COALESCE(t.handle,'someone'),
	       i.status, i.game_id, i.created_at, i.expires_at, i.kind, i.options
	FROM game_invites i
	JOIN users f ON f.id = i.from_id
	JOIN users t ON t.id = i.to_id`

func scanInvite(row pgx.Row) (InviteRow, error) {
	var i InviteRow
	var opts []byte
	err := row.Scan(&i.ID, &i.FromID, &i.FromHandle, &i.ToID, &i.ToHandle,
		&i.Status, &i.GameID, &i.CreatedAt, &i.ExpiresAt, &i.Kind, &opts)
	if err != nil {
		return InviteRow{}, err
	}
	if err := scanOptions(opts, &i.Options); err != nil {
		return InviteRow{}, fmt.Errorf("invite %s: %w", i.ID, err)
	}
	// an invite sent before 0009 offered the 4x4 board, as the column defaults say
	if i.Kind == "constellations" && i.Options.Level == 0 {
		i.Options.Level = legacyLevel
	}
	return i, nil
}

func inviteByID(ctx context.Context, q querier, id string) (InviteRow, error) {
	return scanInvite(q.QueryRow(ctx, inviteSelect+` WHERE i.id = $1::uuid`, id))
}

// CreateInvite opens an invite from one person to another. the opt-in and
// block checks sit in the INSERT itself, as in SendPeerMessage, so nothing can
// change between the check and the write. both people must be in the circle:
// an invite from someone who has left it would have nowhere to be answered.
// kind and options are the game it offers, already checked by the caller
func (s *Store) CreateInvite(ctx context.Context, fromID, toID, kind string, o game.Options) (InviteRow, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return InviteRow{}, err
	}
	defer tx.Rollback(ctx)

	// a pending row past its expiry still holds the pair index until cron runs;
	// clear it here so a new invite is never refused by one nobody can answer
	if _, err := tx.Exec(ctx, `
		UPDATE game_invites SET status = 'expired'
		WHERE status = 'pending' AND expires_at <= now()
		  AND LEAST(from_id, to_id) = LEAST($1::uuid, $2::uuid)
		  AND GREATEST(from_id, to_id) = GREATEST($1::uuid, $2::uuid)`, fromID, toID); err != nil {
		return InviteRow{}, err
	}

	var recent int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM game_invites
		WHERE from_id = $1::uuid AND created_at > now() - interval '1 hour'`, fromID).Scan(&recent); err != nil {
		return InviteRow{}, err
	}
	if recent >= invitesPerHour {
		return InviteRow{}, ErrRateLimited
	}

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO game_invites (from_id, to_id, kind, options)
		SELECT $1::uuid, $2::uuid, $3, $4::jsonb
		WHERE $1::uuid <> $2::uuid
		  AND EXISTS (SELECT 1 FROM users WHERE id = $1::uuid AND peer_opt_in AND handle IS NOT NULL)
		  AND EXISTS (SELECT 1 FROM users WHERE id = $2::uuid AND peer_opt_in AND handle IS NOT NULL)
		  AND NOT EXISTS (SELECT 1 FROM peer_blocks
		                  WHERE (blocker_id = $2::uuid AND blocked_id = $1::uuid)
		                     OR (blocker_id = $1::uuid AND blocked_id = $2::uuid))
		RETURNING id`, fromID, toID, kind, optionsJSON(o)).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return InviteRow{}, ErrPeerUnavailable
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == pendingPairIndex {
		return InviteRow{}, ErrInvitePending
	}
	if err != nil {
		return InviteRow{}, err
	}
	inv, err := inviteByID(ctx, tx, id)
	if err != nil {
		return InviteRow{}, err
	}
	return inv, tx.Commit(ctx)
}

// ListInvites is every open invite involving the person, either direction
func (s *Store) ListInvites(ctx context.Context, userID string) ([]InviteRow, error) {
	rows, err := s.pool.Query(ctx, inviteSelect+`
		WHERE (i.from_id = $1::uuid OR i.to_id = $1::uuid)
		  AND i.status = 'pending' AND i.expires_at > now()
		ORDER BY i.created_at DESC
		LIMIT 50`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []InviteRow{}
	for rows.Next() {
		i, err := scanInvite(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// AcceptInvite turns an invite into a game in one transaction: the invite row
// is locked first, so two tabs accepting at once make one game, not two.
// only the recipient may accept. the sender is player_a. start deals the game
// from the locked invite (the store knows no rules): it returns the new row's
// id, kind, level, options and state, and the seat to move first
type StartFunc func(inv InviteRow) (GameRow, int8, error)

func (s *Store) AcceptInvite(ctx context.Context, userID, inviteID string, start StartFunc) (InviteRow, GameRow, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return InviteRow{}, GameRow{}, err
	}
	defer tx.Rollback(ctx)

	var fromID, toID, status string
	var live bool
	err = tx.QueryRow(ctx, `
		SELECT from_id, to_id, status, expires_at > now()
		FROM game_invites WHERE id = $1::uuid FOR UPDATE`, inviteID).Scan(&fromID, &toID, &status, &live)
	if errors.Is(err, pgx.ErrNoRows) {
		return InviteRow{}, GameRow{}, ErrNotFound
	}
	if err != nil {
		return InviteRow{}, GameRow{}, err
	}
	if toID != userID {
		// the sender, or a stranger with the id: neither learns anything from a 404
		return InviteRow{}, GameRow{}, ErrNotFound
	}
	if status != "pending" {
		return InviteRow{}, GameRow{}, ErrInviteGone
	}
	if !live {
		if _, err := tx.Exec(ctx, `UPDATE game_invites SET status = 'expired' WHERE id = $1::uuid`, inviteID); err != nil {
			return InviteRow{}, GameRow{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return InviteRow{}, GameRow{}, err
		}
		return InviteRow{}, GameRow{}, ErrInviteGone
	}

	inv, err := inviteByID(ctx, tx, inviteID)
	if err != nil {
		return InviteRow{}, GameRow{}, err
	}
	g, toMove, err := start(inv)
	if err != nil {
		return InviteRow{}, GameRow{}, err
	}
	g.Mode, g.PlayerA, g.PlayerB, g.Status = "peer", fromID, &toID, "active"
	// the block check repeats here: a block placed after the invite was sent
	// must still stop the game, and it reads as unavailable, nothing more
	var gameID string
	err = tx.QueryRow(ctx, `
		INSERT INTO games (id, kind, mode, level, options, player_a, player_b, state, turn)
		SELECT $1::uuid, $2, 'peer', $3, $4::jsonb, $5::uuid, $6::uuid, $7::jsonb, $8::uuid
		WHERE NOT EXISTS (SELECT 1 FROM peer_blocks
		                  WHERE (blocker_id = $5::uuid AND blocked_id = $6::uuid)
		                     OR (blocker_id = $6::uuid AND blocked_id = $5::uuid))
		RETURNING id`, g.ID, g.Kind, g.Level, optionsJSON(g.Options), fromID, toID,
		string(g.State), turnFor(g, toMove)).Scan(&gameID)
	if errors.Is(err, pgx.ErrNoRows) {
		return InviteRow{}, GameRow{}, ErrPeerUnavailable
	}
	if err != nil {
		return InviteRow{}, GameRow{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE game_invites SET status = 'accepted', game_id = $2::uuid WHERE id = $1::uuid`, inviteID, gameID); err != nil {
		return InviteRow{}, GameRow{}, err
	}
	if inv, err = inviteByID(ctx, tx, inviteID); err != nil {
		return InviteRow{}, GameRow{}, err
	}
	g, err = gameByID(ctx, tx, gameID)
	if err != nil {
		return InviteRow{}, GameRow{}, err
	}
	return inv, g, tx.Commit(ctx)
}

// DeclineInvite closes a pending invite. either side may: from the sender it
// is a cancel. an expired-but-pending invite can be declined too; the result
// is the same closed invite either way
func (s *Store) DeclineInvite(ctx context.Context, userID, inviteID string) (InviteRow, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE game_invites SET status = 'declined'
		WHERE id = $1::uuid AND status = 'pending' AND (from_id = $2::uuid OR to_id = $2::uuid)`, inviteID, userID)
	if err != nil {
		return InviteRow{}, err
	}
	inv, err := inviteByID(ctx, s.pool, inviteID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && inv.FromID != userID && inv.ToID != userID) {
		return InviteRow{}, ErrNotFound
	}
	if err != nil {
		return InviteRow{}, err
	}
	if tag.RowsAffected() == 0 {
		return InviteRow{}, ErrInviteGone
	}
	return inv, nil
}

// ExpireInvites is the cron sweep for invites nobody answered
func (s *Store) ExpireInvites(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE game_invites SET status = 'expired' WHERE status = 'pending' AND expires_at <= now()`)
	return tag.RowsAffected(), err
}

// ------------------------------------------------------------------ games

// GameRow is a game as stored. PlayerB and HandleB are nil outside peer
// games; DailyKey is set only for a daily puzzle
type GameRow struct {
	ID        string
	Kind      string
	Mode      string
	Level     int
	Options   game.Options
	DailyKey  *string
	PlayerA   string
	HandleA   string
	PlayerB   *string
	HandleB   *string
	State     game.State
	Status    string
	Winner    *string
	Version   int
	UpdatedAt time.Time
}

const gameSelect = `
	SELECT g.id, g.kind, g.mode, g.level, g.options, g.daily_key, g.player_a, COALESCE(a.handle,'someone'),
	       g.player_b, CASE WHEN g.player_b IS NULL THEN NULL ELSE COALESCE(b.handle,'someone') END,
	       g.state, g.status, g.winner, g.version, g.updated_at
	FROM games g
	JOIN users a ON a.id = g.player_a
	LEFT JOIN users b ON b.id = g.player_b`

func scanGame(row pgx.Row) (GameRow, error) {
	var g GameRow
	var opts, st []byte
	err := row.Scan(&g.ID, &g.Kind, &g.Mode, &g.Level, &opts, &g.DailyKey, &g.PlayerA, &g.HandleA,
		&g.PlayerB, &g.HandleB, &st, &g.Status, &g.Winner, &g.Version, &g.UpdatedAt)
	if err != nil {
		return GameRow{}, err
	}
	if err := scanOptions(opts, &g.Options); err != nil {
		return GameRow{}, fmt.Errorf("game %s: %w", g.ID, err)
	}
	// a row written before options existed knows its level only by the column
	if g.Options.Level == 0 {
		g.Options.Level = g.Level
	}
	g.State = game.State(st)
	return g, nil
}

func gameByID(ctx context.Context, q querier, id string) (GameRow, error) {
	return scanGame(q.QueryRow(ctx, gameSelect+` WHERE g.id = $1::uuid`, id))
}

// turnFor is the turn column for a game whose engine says toMove. it is the
// person whenever a person is to move: in astralis that is always player_a,
// as the bot never rests on its own turn, and in solo the one person holds
// every seat. seat 0 (either may move, as in a words duel) leaves a peer game
// on nobody's turn in particular
func turnFor(g GameRow, toMove int8) *string {
	switch {
	case g.Status != "active":
		return nil
	case g.Mode != "peer":
		return &g.PlayerA
	case toMove == 1:
		return &g.PlayerA
	case toMove == 2:
		return g.PlayerB
	}
	return nil
}

// GetGame returns a game only to someone playing in it
func (s *Store) GetGame(ctx context.Context, userID, gameID string) (GameRow, error) {
	g, err := scanGame(s.pool.QueryRow(ctx, gameSelect+`
		WHERE g.id = $1::uuid AND (g.player_a = $2::uuid OR g.player_b = $2::uuid)`, gameID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return GameRow{}, ErrNotFound
	}
	return g, err
}

// CreateGame stores a new astralis or solo game. the caller supplies the id,
// so a bot can be seeded from it, and any opening bot moves already applied to
// the state, so the game is written once. peer games come only from
// AcceptInvite.
//
// a daily puzzle (DailyKey set) is get-or-create on the daily unique index:
// when today's is already there, that row comes back with created false and
// the state passed in is dropped, so two tabs opening the daily at once share
// one game
func (s *Store) CreateGame(ctx context.Context, g GameRow, toMove int8) (GameRow, bool, error) {
	if g.Mode != "astralis" && g.Mode != "solo" {
		return GameRow{}, false, fmt.Errorf("create game: mode %q", g.Mode)
	}
	g.Status = "active"
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO games (id, kind, mode, level, options, daily_key, player_a, state, turn)
		VALUES ($1::uuid, $2, $3, $4, $5::jsonb, $6, $7::uuid, $8::jsonb, $9::uuid)
		ON CONFLICT (player_a, kind, daily_key, (options->>'topic'), (options->>'mode'), level)
		   WHERE daily_key IS NOT NULL DO NOTHING
		RETURNING id`, g.ID, g.Kind, g.Mode, g.Level, optionsJSON(g.Options), g.DailyKey, g.PlayerA,
		string(g.State), turnFor(g, toMove)).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) && g.DailyKey != nil {
		existing, err := scanGame(s.pool.QueryRow(ctx, gameSelect+`
			WHERE g.player_a = $1::uuid AND g.kind = $2 AND g.daily_key = $3
			  AND g.options->>'topic' IS NOT DISTINCT FROM $4::text
			  AND g.options->>'mode' IS NOT DISTINCT FROM $5::text
			  AND g.level = $6`, g.PlayerA, g.Kind, *g.DailyKey, g.Options.Topic, g.Options.Mode, g.Level))
		return existing, false, err
	}
	if err != nil {
		return GameRow{}, false, err
	}
	created, err := gameByID(ctx, s.pool, id)
	return created, true, err
}

// SaveMove writes a new state under an optimistic lock. the version and the
// status both sit in the WHERE, so a second tab, a double click, or a cron
// abandon that landed first all come back as ErrStale and nothing is written.
// toMove is the engine's ToMove for the new state, for the turn column
func (s *Store) SaveMove(ctx context.Context, g GameRow, version int, toMove int8) (GameRow, error) {
	err := s.pool.QueryRow(ctx, `
		UPDATE games
		   SET state = $3::jsonb, turn = $4::uuid, status = $5, winner = $6::uuid,
		       version = version + 1, updated_at = now()
		 WHERE id = $1::uuid AND version = $2 AND status = 'active'
		RETURNING version, updated_at`,
		g.ID, version, string(g.State), turnFor(g, toMove), g.Status, g.Winner).Scan(&g.Version, &g.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return GameRow{}, ErrStale
	}
	return g, err
}

// ActiveGames lists a person's games in play, every kind and mode, most
// recently moved first
func (s *Store) ActiveGames(ctx context.Context, userID string) ([]GameRow, error) {
	rows, err := s.pool.Query(ctx, gameSelect+`
		WHERE (g.player_a = $1::uuid OR g.player_b = $1::uuid) AND g.status = 'active'
		ORDER BY g.updated_at DESC
		LIMIT 50`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GameRow{}
	for rows.Next() {
		g, err := scanGame(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// Progress is what the constellations unlock is derived from: the highest
// level the person has finished against astralis or alone, and the highest
// finished in any mode, peer games included (as either player). 0 for none
func (s *Store) Progress(ctx context.Context, userID string) (botOrSolo, anyMode int, err error) {
	err = s.pool.QueryRow(ctx, `
		SELECT COALESCE(max(level) FILTER (WHERE mode IN ('astralis','solo')), 0)::int,
		       COALESCE(max(level), 0)::int
		FROM games
		WHERE kind = 'constellations' AND status = 'finished'
		  AND (player_a = $1::uuid OR player_b = $1::uuid)`, userID).Scan(&botOrSolo, &anyMode)
	return botOrSolo, anyMode, err
}

// AbandonStaleGames ends games nobody has touched for idle. no winner, no
// points. a solo words or sudoku puzzle is left alone: there is nobody waiting
// on it, and people come back to a puzzle days later
func (s *Store) AbandonStaleGames(ctx context.Context, idle time.Duration) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE games SET status = 'abandoned', turn = NULL, version = version + 1, updated_at = now()
		WHERE status = 'active' AND updated_at < now() - $1::interval
		  AND NOT (mode = 'solo' AND kind IN ('words','sudoku'))`, idle)
	return tag.RowsAffected(), err
}

// ------------------------------------------------------------------ leaderboard

type LeaderRow struct {
	Handle string `json:"handle"`
	Points int    `json:"points"`
	Games  int    `json:"games"`
	Me     bool   `json:"me"`
}

type MyPoints struct {
	Points  int  `json:"points"`
	Games   int  `json:"games"`
	OptedIn bool `json:"optedIn"`
}

// weekPoints is 3 for a win, 2 each for a draw and 1 for finishing, peer games
// of every kind, over the week so far. a finished peer game with no winner is
// a draw: the winner column is only ever cleared by a draw, as deleting a
// player deletes the game. weeks start on Monday in Nairobi time, whatever the
// server's zone
const weekPoints = `
	WITH week AS (
	  SELECT g.player_a, g.player_b, g.winner FROM games g
	  WHERE g.status = 'finished' AND g.mode = 'peer'
	    AND g.updated_at >= date_trunc('week', now() AT TIME ZONE 'Africa/Nairobi') AT TIME ZONE 'Africa/Nairobi'
	), pts AS (
	  SELECT player_a AS uid,
	         CASE WHEN winner IS NULL THEN 2 WHEN winner = player_a THEN 3 ELSE 1 END AS p FROM week
	  UNION ALL
	  SELECT player_b,
	         CASE WHEN winner IS NULL THEN 2 WHEN winner = player_b THEN 3 ELSE 1 END      FROM week
	)`

// Leaderboard is the opted-in top 20, plus the viewer's own tally, which they
// see whether or not they have chosen to be listed
func (s *Store) Leaderboard(ctx context.Context, userID string) ([]LeaderRow, MyPoints, error) {
	rows, err := s.pool.Query(ctx, weekPoints+`
		SELECT u.handle, sum(p)::int AS points, count(*)::int AS games, u.id = $1::uuid
		FROM pts JOIN users u ON u.id = pts.uid
		WHERE u.leaderboard_opt_in AND u.handle IS NOT NULL
		GROUP BY u.id, u.handle
		ORDER BY points DESC, games DESC, u.handle
		LIMIT 20`, userID)
	if err != nil {
		return nil, MyPoints{}, err
	}
	defer rows.Close()
	top := []LeaderRow{}
	for rows.Next() {
		var r LeaderRow
		if err := rows.Scan(&r.Handle, &r.Points, &r.Games, &r.Me); err != nil {
			return nil, MyPoints{}, err
		}
		top = append(top, r)
	}
	if err := rows.Err(); err != nil {
		return nil, MyPoints{}, err
	}

	var me MyPoints
	err = s.pool.QueryRow(ctx, weekPoints+`
		SELECT COALESCE((SELECT sum(p) FROM pts WHERE uid = $1::uuid), 0)::int,
		       (SELECT count(*) FROM pts WHERE uid = $1::uuid)::int,
		       (SELECT leaderboard_opt_in FROM users WHERE id = $1::uuid)`, userID).
		Scan(&me.Points, &me.Games, &me.OptedIn)
	return top, me, err
}

func (s *Store) SetLeaderboardOptIn(ctx context.Context, userID string, in bool) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET leaderboard_opt_in = $2 WHERE id = $1::uuid`, userID, in)
	return err
}
