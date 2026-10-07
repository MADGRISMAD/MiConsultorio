-- Two-step verification (TOTP, RFC 6238) and the clinic's policy about it.
-- The secret is stored encrypted (AES-GCM, key derived from TOKEN_ENC_KEY); recovery codes only as salted hashes.

ALTER TABLE users
    ADD COLUMN totp_secret_enc    bytea,
    ADD COLUMN totp_enabled       boolean NOT NULL DEFAULT false,
    ADD COLUMN totp_confirmed_at  timestamptz,
    ADD COLUMN totp_last_step     bigint NOT NULL DEFAULT 0; -- last accepted 30 s step: a code works only once

CREATE TABLE totp_recovery_codes (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    clinic_id  uuid REFERENCES clinics (id) ON DELETE CASCADE, -- null for platform staff
    salt       text NOT NULL,
    code_hash  text NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX totp_recovery_codes_user_idx ON totp_recovery_codes (user_id) WHERE used_at IS NULL;

-- Who must use two-step verification: none, admins, clinical (admins and doctors) or all.
ALTER TABLE clinics
    ADD COLUMN require_2fa text NOT NULL DEFAULT 'none' CHECK (require_2fa IN ('none', 'admins', 'clinical', 'all'));
