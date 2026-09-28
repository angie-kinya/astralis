-- Astralis 0003 — email sign-in, owned end to end by the Go service.
--
-- GitHub sign-in is gone: it excluded everyone without a GitHub account,
-- which is most of the people this is for. Identity is now an email address
-- whose ownership has been PROVEN by a one-time code. An unverified email is
-- not an identity — anyone could type yours.
--
-- Two properties this schema exists to guarantee:
--   1. The email is never exposed to another user. It lives in exactly one
--      column, no view or peer query selects it, and the only endpoint that
--      returns it is /me — to its owner.
--   2. Sign-out and erasure actually end access. Sessions are rows, not
--      self-contained tokens, so deleting the row ends the session at once.
--      A stateless token would stay valid until it expired — and after an
--      account deletion it could even recreate the user.

-- ---------------------------------------------------------------- identity
-- external_id held the GitHub subject. Nothing sets it now; UNIQUE still
-- permits many NULLs, so existing rows are untouched.
ALTER TABLE users ALTER COLUMN external_id DROP NOT NULL;
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified_at TIMESTAMPTZ;
-- Optional, default off. Release notes only — never marketing, never shared.
ALTER TABLE users ADD COLUMN IF NOT EXISTS notify_releases BOOLEAN NOT NULL DEFAULT false;

-- ---------------------------------------------------------------- login codes
-- A code is stored only as an HMAC keyed by the server secret, so a database
-- leak does not leak working codes. Codes live ten minutes, allow five
-- guesses, and requesting a new one burns every older one.
CREATE TABLE IF NOT EXISTS login_codes (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email      TEXT NOT NULL,
    code_hash  TEXT NOT NULL,
    attempts   SMALLINT NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_login_codes_email ON login_codes (email, created_at DESC);

-- ---------------------------------------------------------------- sessions
-- The cookie holds a random 256-bit token; this table holds only its SHA-256.
-- Someone who reads the database cannot sign in as anybody.
CREATE TABLE IF NOT EXISTS auth_sessions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash   TEXT NOT NULL UNIQUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at   TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_user ON auth_sessions (user_id);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_expiry ON auth_sessions (expires_at);
