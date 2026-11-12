-- Astralis 0007: circle v2. delete, block and report.
--
-- all three are required before anything invites strangers to interact more.
-- blocking is enforced inside the same statement that sends (store.SendPeerMessage),
-- like the opt-in check, so there is no race between a block and a send.

-- deletion erases the content and keeps a tombstone, so the thread still makes sense
ALTER TABLE peer_messages ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE peer_messages ALTER COLUMN body DROP NOT NULL;
DO $$ BEGIN
    ALTER TABLE peer_messages ADD CONSTRAINT peer_body_or_deleted
        CHECK (body IS NOT NULL OR deleted_at IS NOT NULL);
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS peer_blocks (
    blocker_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    blocked_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (blocker_id, blocked_id),
    CHECK (blocker_id <> blocked_id)
);
-- the reverse direction is checked on every send and list
CREATE INDEX IF NOT EXISTS idx_peer_blocks_blocked ON peer_blocks (blocked_id, blocker_id);

-- a report keeps a snapshot, so deleting a message cannot erase the evidence.
-- the snapshot is message content kept for moderation; the consent sheet says so
CREATE TABLE IF NOT EXISTS peer_reports (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reporter_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reported_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    peer_message_id UUID REFERENCES peer_messages(id) ON DELETE SET NULL,
    body_snapshot   TEXT,
    reason          TEXT NOT NULL CHECK (reason IN ('harassment','spam','unsafe','other')),
    reviewed        BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_peer_reports_unreviewed ON peer_reports (reviewed, created_at DESC);
