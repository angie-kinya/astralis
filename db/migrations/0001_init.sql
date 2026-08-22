-- Astralis initial schema
-- Run: psql "$DATABASE_URL" -f db/migrations/0001_init.sql

CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ---------------------------------------------------------------- identity
CREATE TABLE IF NOT EXISTS users (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id      TEXT UNIQUE NOT NULL,          -- subject from Auth.js
    email            TEXT UNIQUE NOT NULL,
    display_name     TEXT,
    consent_version  TEXT,                          -- NULL until consent given
    consent_given_at TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at       TIMESTAMPTZ                    -- soft delete = right to erasure
);

-- ---------------------------------------------------------------- conversation
CREATE TABLE IF NOT EXISTS sessions (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title            TEXT,
    started_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_activity_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at         TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id, last_activity_at DESC);

DO $$ BEGIN
    CREATE TYPE message_role AS ENUM ('user','assistant','system');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS messages (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id     UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    role           message_role NOT NULL,
    content        TEXT NOT NULL,
    crisis_flagged BOOLEAN NOT NULL DEFAULT false,
    safety_score   REAL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id, created_at);

-- ---------------------------------------------------------------- knowledge base
CREATE TABLE IF NOT EXISTS research_docs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source      TEXT NOT NULL,                      -- 'openalex' | 'semanticscholar'
    external_id TEXT NOT NULL,
    title       TEXT NOT NULL,
    authors     TEXT[],
    year        INT,
    url         TEXT,
    abstract    TEXT,
    fetched_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source, external_id)
);

-- helpful_weight is the *self-improvement* dial: user feedback nudges it and
-- retrieval multiplies similarity by it. No model retraining required.
CREATE TABLE IF NOT EXISTS research_chunks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    doc_id          UUID NOT NULL REFERENCES research_docs(id) ON DELETE CASCADE,
    chunk_index     INT NOT NULL,
    content         TEXT NOT NULL,
    embedding       VECTOR(1536) NOT NULL,
    helpful_weight  REAL NOT NULL DEFAULT 1.0,
    times_retrieved INT  NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (doc_id, chunk_index)
);
CREATE INDEX IF NOT EXISTS idx_chunks_embedding
    ON research_chunks USING hnsw (embedding vector_cosine_ops);

-- ---------------------------------------------------------------- episodic memory
CREATE TABLE IF NOT EXISTS session_summaries (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL UNIQUE REFERENCES sessions(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    summary    TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_summaries_user ON session_summaries(user_id, created_at DESC);

-- ---------------------------------------------------------------- feedback loop
CREATE TABLE IF NOT EXISTS recommendations (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id       UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    message_id       UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    source_chunk_ids UUID[] NOT NULL DEFAULT '{}',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS feedback (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recommendation_id UUID NOT NULL REFERENCES recommendations(id) ON DELETE CASCADE,
    user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rating            SMALLINT NOT NULL CHECK (rating BETWEEN -1 AND 1),
    comment           TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (recommendation_id, user_id)
);

-- ---------------------------------------------------------------- safety audit
CREATE TABLE IF NOT EXISTS crisis_flags (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id       UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    matched_rule     TEXT,
    classifier_score REAL,
    reviewed         BOOLEAN NOT NULL DEFAULT false,
    reviewed_by      TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_crisis_unreviewed ON crisis_flags(reviewed, created_at DESC);
