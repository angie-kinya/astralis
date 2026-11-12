-- Astralis 0008: constellations, a small two-player game inside the circle.
--
-- dots and boxes on a 4x4 star field. the rules live in pure go
-- (internal/game); the database only stores state and arbitrates races: a
-- move is saved with an optimistic lock on games.version, so two fast clicks
-- or two tabs can never both land.
--
-- three modes. 'peer' is two people in the circle. 'astralis' is one person
-- against a deterministic go opponent (internal/game, no model involved).
-- 'solo' is one device, both colours: practice, or two people sharing a phone.
-- only peer games count towards the leaderboard, so it stays about people.
--
-- the leaderboard is gentle by design: opt-in visibility, weekly reset,
-- points for finishing as well as winning, no streaks or loss counts. the
-- agent never mentions the game.

ALTER TABLE users ADD COLUMN IF NOT EXISTS leaderboard_opt_in BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE IF NOT EXISTS games (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind        TEXT NOT NULL DEFAULT 'constellations',
    mode        TEXT NOT NULL DEFAULT 'peer' CHECK (mode IN ('peer','astralis','solo')),
    player_a    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    player_b    UUID REFERENCES users(id) ON DELETE CASCADE,  -- NULL unless mode = 'peer'
    state       JSONB NOT NULL,
    turn        UUID,                                   -- whose move; NULL when over
    status      TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','finished','abandoned')),
    winner      UUID REFERENCES users(id) ON DELETE SET NULL,
    version     INT  NOT NULL DEFAULT 0,                -- optimistic lock
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (player_a <> player_b),
    -- a second person exists exactly when it is a peer game
    CHECK ((mode = 'peer') = (player_b IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS idx_games_finished ON games (status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_games_player_a ON games (player_a, status);
CREATE INDEX IF NOT EXISTS idx_games_player_b ON games (player_b, status);

CREATE TABLE IF NOT EXISTS game_invites (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    from_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    to_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status     TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','accepted','declined','expired')),
    game_id    UUID REFERENCES games(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT now() + interval '10 minutes',
    CHECK (from_id <> to_id)
);
-- one pending invite per pair, whichever of them sent it. stale pending rows
-- are flipped to 'expired' inside the same transaction that inserts a new one
CREATE UNIQUE INDEX IF NOT EXISTS uq_game_invites_pending_pair
    ON game_invites (LEAST(from_id, to_id), GREATEST(from_id, to_id))
    WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_game_invites_to   ON game_invites (to_id, status);
-- the five-per-hour limit counts a sender's recent invites
CREATE INDEX IF NOT EXISTS idx_game_invites_from ON game_invites (from_id, created_at DESC);
