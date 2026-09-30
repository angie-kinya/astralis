-- Astralis 0005 — consent is a timestamp, not a version label.
--
-- consent_given_at already records WHEN someone agreed. The version string
-- only existed to re-ask people after the terms change, and a date does that
-- job better: the API compares consent_given_at with the date the terms last
-- changed (TermsUpdatedAt in internal/api/handlers.go). Bump that one date and
-- everyone who agreed earlier is asked again.
--
-- Before dropping the column, keep the meaning it carried: a consent to an
-- OLDER version is not a consent to the current terms, so those timestamps
-- are cleared and those people will be asked again. Consents to the current
-- version ('2026-10-v2') keep their timestamp untouched.
--
-- Guarded so it is idempotent: after the first run the column is gone and the
-- block does nothing.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_name = 'users' AND column_name = 'consent_version') THEN
        UPDATE users SET consent_given_at = NULL
        WHERE consent_version IS DISTINCT FROM '2026-10-v2';
        ALTER TABLE users DROP COLUMN consent_version;
    END IF;
END $$;
