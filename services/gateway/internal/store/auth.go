package store

import (
	"context"
	"crypto/subtle"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrNoSession means the cookie was missing, unknown, or expired. The caller
// cannot tell which, and should not be able to: all three mean "sign in".
var ErrNoSession = errors.New("no valid session")

// ------------------------------------------------------------------ codes

// CodesSince counts codes issued to an address recently — the per-inbox cap
// that stops the sign-in form being used to flood somebody else's email.
func (s *Store) CodesSince(ctx context.Context, email string, since time.Time) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM login_codes WHERE email=$1 AND created_at > $2`, email, since).Scan(&n)
	return n, err
}

// IssueLoginCode stores a new code and burns every older unused one in the
// same transaction. Only the most recent code ever works — so a code from an
// old email sitting in an inbox is worthless.
func (s *Store) IssueLoginCode(ctx context.Context, email, codeHash string, ttl time.Duration) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`UPDATE login_codes SET used_at=now() WHERE email=$1 AND used_at IS NULL`, email); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO login_codes (email, code_hash, expires_at)
		VALUES ($1, $2, now() + $3::interval)`, email, codeHash, ttl); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ConsumeLoginCode checks a guess against the live code for an address.
//
// The row is locked FOR UPDATE, so two simultaneous guesses cannot both read
// "four attempts used" and both get a fifth. The comparison is constant-time
// so response timing leaks nothing about how close a guess was. The fifth
// wrong guess burns the code outright.
func (s *Store) ConsumeLoginCode(ctx context.Context, email, codeHash string, maxGuesses int) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	var id, stored string
	var attempts int
	err = tx.QueryRow(ctx, `
		SELECT id, code_hash, attempts FROM login_codes
		WHERE email=$1 AND used_at IS NULL AND expires_at > now()
		ORDER BY created_at DESC LIMIT 1
		FOR UPDATE`, email).Scan(&id, &stored, &attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if subtle.ConstantTimeCompare([]byte(stored), []byte(codeHash)) == 1 {
		if _, err := tx.Exec(ctx, `UPDATE login_codes SET used_at=now() WHERE id=$1`, id); err != nil {
			return false, err
		}
		return true, tx.Commit(ctx)
	}

	attempts++
	if attempts >= maxGuesses {
		_, err = tx.Exec(ctx, `UPDATE login_codes SET attempts=$2, used_at=now() WHERE id=$1`, id, attempts)
	} else {
		_, err = tx.Exec(ctx, `UPDATE login_codes SET attempts=$2 WHERE id=$1`, id, attempts)
	}
	if err != nil {
		return false, err
	}
	return false, tx.Commit(ctx)
}

// ------------------------------------------------------------------ users

// UserForVerifiedEmail returns the account for an address that has just been
// proven, creating it on first sign-in.
//
// notifyReleases applies only to a NEW account. A returning person does not
// know whether they are returning when they see the checkbox, so honouring it
// on every sign-in would silently overwrite the preference they set before.
// Returning users change it under "You".
func (s *Store) UserForVerifiedEmail(ctx context.Context, email string, notifyReleases bool) (User, error) {
	return scanUser(s.pool.QueryRow(ctx, `
		INSERT INTO users (email, email_verified_at, notify_releases)
		VALUES ($1, now(), $2)
		ON CONFLICT (email) DO UPDATE SET email_verified_at = now()
		RETURNING `+userCols, email, notifyReleases))
}

func (s *Store) SetNotifyReleases(ctx context.Context, userID string, on bool) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET notify_releases=$2 WHERE id=$1`, userID, on)
	return err
}

// ReleaseSubscribers is the only query in the codebase that reads email
// addresses in bulk, and it exists solely for cmd/announce.
func (s *Store) ReleaseSubscribers(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT email FROM users WHERE notify_releases = true AND email_verified_at IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ------------------------------------------------------------------ sessions

func (s *Store) CreateAuthSession(ctx context.Context, userID, tokenHash string, ttl time.Duration) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO auth_sessions (user_id, token_hash, expires_at)
		VALUES ($1, $2, now() + $3::interval)`, userID, tokenHash, ttl)
	return err
}

// UserBySession resolves a cookie to a user in one round trip, and records
// presence in the same statement: the session's last_seen_at and the user's
// last_seen_at (which is what "online" in the circle reads) both move.
func (s *Store) UserBySession(ctx context.Context, tokenHash string) (User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx, `
		WITH sess AS (
			UPDATE auth_sessions SET last_seen_at = now()
			WHERE token_hash = $1 AND expires_at > now()
			RETURNING user_id
		)
		UPDATE users SET last_seen_at = now()
		WHERE id = (SELECT user_id FROM sess)
		RETURNING `+userCols, tokenHash))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNoSession
	}
	return u, err
}

func (s *Store) DeleteAuthSession(ctx context.Context, tokenHash string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM auth_sessions WHERE token_hash=$1`, tokenHash)
	return err
}

// PurgeExpiredAuth removes dead sessions and codes. Run from cmd/summarise,
// which is already the periodic job; nothing correct depends on it running,
// since every lookup checks expiry anyway. It is housekeeping, not security.
func (s *Store) PurgeExpiredAuth(ctx context.Context) (int64, error) {
	var total int64
	for _, q := range []string{
		`DELETE FROM auth_sessions WHERE expires_at < now()`,
		`DELETE FROM login_codes WHERE created_at < now() - interval '1 day'`,
	} {
		tag, err := s.pool.Exec(ctx, q)
		if err != nil {
			return total, err
		}
		total += tag.RowsAffected()
	}
	return total, nil
}
