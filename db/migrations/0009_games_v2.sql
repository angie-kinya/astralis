-- Astralis 0009: one game model for every game.
--
-- games.kind already existed (default 'constellations'); now it means
-- something. each kind is a pure engine in internal/game/<kind>, and games.state
-- is whatever json that engine writes. options pick the variant (level, topic,
-- mode, daily date) and are kept beside the state so the ui can show them
-- without asking the engine.
--
-- levels unlock by finishing the level before (derived from finished games,
-- so there is no progress table to drift out of sync).
--
-- old rows stay valid: kind defaults to 'constellations', level to 2 (the
-- 4x4 board every existing game was played on).

ALTER TABLE games
    ADD COLUMN IF NOT EXISTS level     INT   NOT NULL DEFAULT 2,
    ADD COLUMN IF NOT EXISTS options   JSONB NOT NULL DEFAULT '{}',
    -- set only for a daily puzzle ('2026-11-14'), so a day's puzzle resumes
    -- instead of being dealt again
    ADD COLUMN IF NOT EXISTS daily_key TEXT;

DO $$ BEGIN
    ALTER TABLE games ADD CONSTRAINT games_kind_known
        CHECK (kind IN ('constellations','words','sudoku'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- only constellations has an astralis opponent; sudoku is always played alone
DO $$ BEGIN
    ALTER TABLE games ADD CONSTRAINT games_mode_fits_kind
        CHECK ((mode <> 'astralis' OR kind = 'constellations') AND (kind <> 'sudoku' OR mode = 'solo'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- one daily puzzle per person per kind and variant per day
CREATE UNIQUE INDEX IF NOT EXISTS uq_games_daily
    ON games (player_a, kind, daily_key, (options->>'topic'), (options->>'mode'), level)
    WHERE daily_key IS NOT NULL;
-- the unlock query: highest finished level per person and kind
CREATE INDEX IF NOT EXISTS idx_games_progress ON games (player_a, kind, level) WHERE status = 'finished';

ALTER TABLE game_invites
    ADD COLUMN IF NOT EXISTS kind    TEXT  NOT NULL DEFAULT 'constellations',
    ADD COLUMN IF NOT EXISTS options JSONB NOT NULL DEFAULT '{}';

DO $$ BEGIN
    ALTER TABLE game_invites ADD CONSTRAINT game_invites_kind_peer
        CHECK (kind IN ('constellations','words'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
