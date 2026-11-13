-- Astralis 0010: agent v2.2. a stance per turn, and replies that are never cut off.
--
-- every column is on agent_runs, which cascades from users, so erasure is
-- still one statement. topic is the analyser's short phrase for what the
-- conversation is about; it is read back only to follow the thread within a
-- session (store.LastTopic) and is never shown on the admin pages.
--
-- idempotent: safe to run twice, like every migration here.

ALTER TABLE agent_runs
    ADD COLUMN IF NOT EXISTS stance        TEXT,                           -- listen | explore | advise | inform | greet | off_topic
    ADD COLUMN IF NOT EXISTS band_min      INT,                            -- the length band in words, after mirroring
    ADD COLUMN IF NOT EXISTS band_max      INT,
    ADD COLUMN IF NOT EXISTS advice        TEXT,                           -- none | offer | allow | give
    ADD COLUMN IF NOT EXISTS cite_mode     TEXT,                           -- none | words | markers
    ADD COLUMN IF NOT EXISTS truncated     BOOLEAN NOT NULL DEFAULT false, -- the shipped reply lost a sentence to the ceiling
    ADD COLUMN IF NOT EXISTS used_in_words BOOLEAN NOT NULL DEFAULT false, -- research drawn on without markers (heuristic)
    ADD COLUMN IF NOT EXISTS topic         TEXT,
    ADD COLUMN IF NOT EXISTS topic_turns   INT     NOT NULL DEFAULT 0;

-- LastTopic reads a session's newest run on every turn
CREATE INDEX IF NOT EXISTS idx_agent_runs_session ON agent_runs (session_id, created_at DESC);
