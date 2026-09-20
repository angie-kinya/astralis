-- Astralis 0002 — identity, peer support, notes.
--
-- Three additions, all driven by the same principle as 0001: a person should
-- be able to use Astralis without ever telling it who they are.
--
--   handle       an anonymous display name, chosen in onboarding. This is the
--                ONLY identifier other users ever see. email and display_name
--                come from GitHub and are never exposed to a peer.
--   peer_opt_in  talking to other people is opt-in and reversible. Default
--                false: nobody joins a support circle by accident.
--   last_seen_at presence. Updated on every authenticated request; "online"
--                is a read-time comparison, not a stored flag, so a crashed
--                browser cannot leave someone showing as online forever.

-- ---------------------------------------------------------------- identity
ALTER TABLE users ADD COLUMN IF NOT EXISTS handle       TEXT UNIQUE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS peer_opt_in  BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now();

CREATE INDEX IF NOT EXISTS idx_users_peers ON users (peer_opt_in, last_seen_at DESC)
    WHERE peer_opt_in = true;

-- ---------------------------------------------------------------- peer messages
-- Direct messages between two people. There is no thread table: a thread IS
-- the unordered pair of participants, which means there is no way to create a
-- thread that belongs to nobody, and no join to get wrong.
CREATE TABLE IF NOT EXISTS peer_messages (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipient_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body         TEXT NOT NULL,
    -- Peer messages run through the same safety filter as agent messages.
    -- Flagged messages are never delivered; the sender gets the resource card.
    crisis_flagged BOOLEAN NOT NULL DEFAULT false,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    read_at      TIMESTAMPTZ,
    CHECK (sender_id <> recipient_id)
);

-- Covers both directions of a conversation lookup.
CREATE INDEX IF NOT EXISTS idx_peer_msgs_pair
    ON peer_messages (sender_id, recipient_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_peer_msgs_inbox
    ON peer_messages (recipient_id, read_at, created_at DESC);

-- ---------------------------------------------------------------- notes
-- A private notepad. Never retrieved, never embedded, never sent to any model.
-- The one place in Astralis where what you write goes nowhere at all.
CREATE TABLE IF NOT EXISTS notes (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title      TEXT,
    body       TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_notes_user ON notes (user_id, updated_at DESC);

-- ---------------------------------------------------------------- safety audit
-- crisis_flags originally assumed every flagged message lived in `messages`.
-- Peer messages live in their own table, so the flag needs to point at either
-- one. Making message_id nullable and adding a CHECK is better than dropping
-- the foreign keys: the database still refuses to store a flag that points at
-- nothing, and still refuses to store one pointing at both.
ALTER TABLE crisis_flags ALTER COLUMN message_id DROP NOT NULL;
ALTER TABLE crisis_flags ADD COLUMN IF NOT EXISTS peer_message_id UUID
    REFERENCES peer_messages(id) ON DELETE CASCADE;

DO $$ BEGIN
    ALTER TABLE crisis_flags ADD CONSTRAINT crisis_flags_one_target
        CHECK (num_nonnulls(message_id, peer_message_id) = 1);
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
