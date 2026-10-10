-- Photos of the public page of a clinic (profile picture, cover banner, up to 5 gallery photos, one per specialist),
-- and whether each specialist shows on it.
CREATE TABLE clinic_media (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id  uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    slot       text NOT NULL CHECK (slot IN ('profile', 'cover', 'gallery', 'pro')),
    user_id    uuid REFERENCES users (id) ON DELETE CASCADE,   -- slot 'pro': the specialist in the photo
    mime       text NOT NULL,
    data       bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((slot = 'pro') = (user_id IS NOT NULL))
);
CREATE UNIQUE INDEX clinic_media_profile_key ON clinic_media (clinic_id) WHERE slot = 'profile';
CREATE UNIQUE INDEX clinic_media_cover_key ON clinic_media (clinic_id) WHERE slot = 'cover';
CREATE UNIQUE INDEX clinic_media_pro_key ON clinic_media (user_id) WHERE slot = 'pro';
CREATE INDEX clinic_media_clinic_idx ON clinic_media (clinic_id, slot, created_at);

ALTER TABLE users ADD COLUMN public_hidden boolean NOT NULL DEFAULT false;  -- the specialist does not show on the public page
ALTER TABLE clinic_profile ADD COLUMN contact_email text NOT NULL DEFAULT '';
