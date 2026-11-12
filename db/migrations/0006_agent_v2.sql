-- Astralis 0006: agent v2. the person model, voice variants and the style audit.
--
-- person_facts is new personal data, so it ships with a TermsUpdatedAt bump
-- (internal/api/handlers.go) and a "What Astralis remembers" list in Settings
-- where every row can be deleted. crisis-flagged messages never reach the
-- extractor, so no fact is ever drawn from one.
--
-- every new table cascades from users (directly or through sessions), so
-- erasure stays one statement: DELETE FROM users.

-- ---------------------------------------------------------------- person model
CREATE TABLE IF NOT EXISTS person_facts (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind              TEXT NOT NULL CHECK (kind IN ('person','event','commitment','helped','context')),
    subject           TEXT NOT NULL,             -- 'branch manager', 'month-end audit'
    detail            TEXT NOT NULL,             -- one sentence, in their words where possible
    status            TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','done','dropped')),
    mentions          INT  NOT NULL DEFAULT 1,
    embedding         VECTOR(1536),
    source_session_id UUID REFERENCES sessions(id) ON DELETE SET NULL,
    first_seen        TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen         TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- one row per (person, kind, subject): repeats raise mentions instead of duplicating
CREATE UNIQUE INDEX IF NOT EXISTS uq_person_facts
    ON person_facts (user_id, kind, lower(subject));
-- a handful of rows per person, so recall filters by user and scores in place; no hnsw needed
CREATE INDEX IF NOT EXISTS idx_person_facts_user ON person_facts (user_id, last_seen DESC);

-- ---------------------------------------------------------------- variants
-- a variant is chosen once per session, so the voice never changes mid-conversation
ALTER TABLE sessions        ADD COLUMN IF NOT EXISTS prompt_variant TEXT;
ALTER TABLE recommendations ADD COLUMN IF NOT EXISTS prompt_variant TEXT;

-- the end-of-session card ("was this conversation worth having?"). one deliberate
-- rating per session is better bandit data than a thumb under every reply
ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS worth_it BOOLEAN,
    ADD COLUMN IF NOT EXISTS rated_at TIMESTAMPTZ;

-- ---------------------------------------------------------------- style audit
ALTER TABLE agent_runs
    ADD COLUMN IF NOT EXISTS prompt_variant   TEXT,
    ADD COLUMN IF NOT EXISTS word_budget      INT,
    ADD COLUMN IF NOT EXISTS reply_words      INT,
    ADD COLUMN IF NOT EXISTS style_violations TEXT[]  NOT NULL DEFAULT '{}',  -- first-pass lint
    ADD COLUMN IF NOT EXISTS repaired         BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS facts_used       INT     NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS moves            TEXT[]  NOT NULL DEFAULT '{}';

-- a 'noticing' move fires at most once per session per kind
CREATE TABLE IF NOT EXISTS session_moves (
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    move       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (session_id, move)
);
