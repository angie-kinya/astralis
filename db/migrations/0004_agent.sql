-- Astralis 0004: the agent's working memory and its audit trail.
--
-- Two new tables, one new column. Nothing here stores anything a person wrote.
-- Every row is either a measurement the agent derived from a message, or a
-- record of what the agent did while answering it.
--
--   turn_signals  structured observations per user message (intent, burnout
--                 dimensions, demands, resources). This is the dataset a
--                 prediction model trains on in a later phase. The language
--                 model is never trained on it; it only produces it.
--   agent_runs    one row per chat turn: which route it took, what each stage
--                 cost in milliseconds and tokens, and any error. This is how
--                 production is observed without SSH access to logs.
--
-- Both cascade from users, so DELETE /api/v1/me still erases everything.

-- ---------------------------------------------------------------- signals
CREATE TABLE IF NOT EXISTS turn_signals (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id       UUID NOT NULL UNIQUE REFERENCES messages(id) ON DELETE CASCADE,
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_id       UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    intent           TEXT NOT NULL,
    -- NULL means "no evidence in this message", which is not the same as low.
    -- Keeping that distinction is what makes the column usable as a feature.
    exhaustion       SMALLINT CHECK (exhaustion BETWEEN 1 AND 3),
    cynicism         SMALLINT CHECK (cynicism BETWEEN 1 AND 3),
    reduced_efficacy SMALLINT CHECK (reduced_efficacy BETWEEN 1 AND 3),
    demands          TEXT[] NOT NULL DEFAULT '{}',
    resources        TEXT[] NOT NULL DEFAULT '{}',
    valence          REAL CHECK (valence BETWEEN -1 AND 1),
    -- Which prompt and schema produced the row. Change the analyser, bump
    -- the version, and old and new rows can never be silently mixed.
    analyser_version TEXT NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_turn_signals_user ON turn_signals (user_id, created_at DESC);

-- ---------------------------------------------------------------- runs
CREATE TABLE IF NOT EXISTS agent_runs (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_id       UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    user_message_id  UUID REFERENCES messages(id) ON DELETE CASCADE,
    reply_message_id UUID REFERENCES messages(id) ON DELETE SET NULL,
    route            TEXT NOT NULL,            -- 'agent' | 'crisis' | 'failed'
    intent           TEXT,
    safety_rule      TEXT,
    analyser_ok      BOOLEAN NOT NULL DEFAULT false,
    retrieval_used   BOOLEAN NOT NULL DEFAULT false,
    chunks_found     INT NOT NULL DEFAULT 0,
    chunks_cited     INT NOT NULL DEFAULT 0,
    top_similarity   REAL,
    input_tokens     INT NOT NULL DEFAULT 0,
    output_tokens    INT NOT NULL DEFAULT 0,
    stop_reason      TEXT,
    stages_ms        JSONB NOT NULL DEFAULT '{}',
    total_ms         INT NOT NULL DEFAULT 0,
    error            TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_agent_runs_time ON agent_runs (created_at DESC);

-- ---------------------------------------------------------------- feedback
-- Only the passages the reply actually cited should earn credit or blame
-- from a thumbs up or down. Retrieved but uncited passages played no part.
ALTER TABLE recommendations
    ADD COLUMN IF NOT EXISTS cited_chunk_ids UUID[] NOT NULL DEFAULT '{}';