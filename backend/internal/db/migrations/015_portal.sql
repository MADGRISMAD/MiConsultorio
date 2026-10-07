-- Patient portal: public pages under /portal/<slug> (the clinic's booking slug) where a patient or guardian
-- signs in with a one-time code sent to their e-mail and sees their appointments, recetas and vaccination card.
-- The cancellation margin is agenda_settings.cancel_min_hours (migration 019).

ALTER TABLE agenda_settings
    ADD COLUMN portal_enabled boolean NOT NULL DEFAULT false,
    ADD COLUMN portal_welcome text NOT NULL DEFAULT '';

-- One-time sign-in codes. Only an HMAC of the code is stored; a code works once, for 10 minutes and 5 tries.
CREATE TABLE portal_otps (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id  uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    email      text NOT NULL,
    code_hash  bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    attempts   integer NOT NULL DEFAULT 0,
    used_at    timestamptz,
    ip         text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX portal_otps_lookup_idx ON portal_otps (clinic_id, email, created_at DESC);
