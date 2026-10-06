-- Roles replace per-user permission lists, sign-in works with e-mail or username,
-- clinics get plans/subscriptions, and platform staff live outside any clinic.

-- ---- users ---------------------------------------------------------------
ALTER TABLE users
    ADD COLUMN name          text NOT NULL DEFAULT '',
    ADD COLUMN email         text,
    ADD COLUMN phone         text NOT NULL DEFAULT '',
    ADD COLUMN role          text,
    ADD COLUMN disabled      boolean NOT NULL DEFAULT false,
    ADD COLUMN disabled_at   timestamptz,
    ADD COLUMN token_version integer NOT NULL DEFAULT 0,
    ADD COLUMN last_login_at timestamptz;

UPDATE users SET role = CASE
    WHEN 'adminUsers' = ANY (permissions) THEN 'admin'
    WHEN 'adminHistorials' = ANY (permissions) OR 'navHistorials' = ANY (permissions) THEN 'doctor'
    WHEN 'adminAppointments' = ANY (permissions) OR 'navAppointments' = ANY (permissions) THEN 'reception'
    ELSE 'cashier'
END;
UPDATE users SET name = username WHERE name = '';

-- Usernames become globally unique (they can be used to sign in) and may not contain '@',
-- which is reserved for e-mails. Later duplicates get a short suffix.
UPDATE users SET username = replace(username, '@', '_');
WITH d AS (
    SELECT id, row_number() OVER (PARTITION BY lower(username) ORDER BY created_at, id) AS rn FROM users
)
UPDATE users u SET username = u.username || '_' || substr(u.id::text, 1, 4)
FROM d WHERE d.id = u.id AND d.rn > 1;

-- The oldest admin of each clinic keeps the clinic's e-mail as their own sign-in e-mail.
UPDATE users u SET email = lower(c.email)
FROM clinics c,
     (SELECT DISTINCT ON (clinic_id) id FROM users WHERE role = 'admin' ORDER BY clinic_id, created_at, id) oldest
WHERE u.id = oldest.id AND c.id = u.clinic_id;

ALTER TABLE users ALTER COLUMN role SET NOT NULL;
ALTER TABLE users ALTER COLUMN clinic_id DROP NOT NULL;
ALTER TABLE users ADD CONSTRAINT users_role_check
    CHECK (role IN ('admin', 'doctor', 'reception', 'cashier', 'platform_admin', 'platform_support'));
ALTER TABLE users ADD CONSTRAINT users_scope_check
    CHECK ((role IN ('platform_admin', 'platform_support')) = (clinic_id IS NULL));
ALTER TABLE users ADD CONSTRAINT users_username_check CHECK (position('@' in username) = 0);
ALTER TABLE users DROP CONSTRAINT users_clinic_id_username_key;
CREATE UNIQUE INDEX users_username_key ON users (lower(username));
CREATE UNIQUE INDEX users_email_key ON users (lower(email)) WHERE email IS NOT NULL;
CREATE INDEX users_clinic_idx ON users (clinic_id);
ALTER TABLE users DROP COLUMN permissions;

-- ---- clinics: plan and subscription ---------------------------------------
ALTER TABLE clinics
    ADD COLUMN plan               text NOT NULL DEFAULT 'consultorio'
        CHECK (plan IN ('consultorio', 'clinica', 'empresarial')),
    ADD COLUMN billing_status     text NOT NULL DEFAULT 'active'
        CHECK (billing_status IN ('trialing', 'active', 'past_due', 'suspended')),
    ADD COLUMN trial_ends_at      timestamptz,
    ADD COLUMN current_period_end timestamptz,
    ADD COLUMN suspended_at       timestamptz,
    ADD COLUMN suspended_reason   text NOT NULL DEFAULT '',
    ADD COLUMN updated_at         timestamptz NOT NULL DEFAULT now();
DROP INDEX clinics_email_key; -- the sign-in e-mail now lives on users

-- ---- audit trail and manual payments --------------------------------------
CREATE TABLE activity_log (
    id         bigserial PRIMARY KEY,
    clinic_id  uuid REFERENCES clinics (id) ON DELETE CASCADE,
    actor_id   uuid REFERENCES users (id) ON DELETE SET NULL,
    actor_name text NOT NULL DEFAULT '',
    type       text NOT NULL,
    message    text NOT NULL,
    meta       jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX activity_clinic_idx ON activity_log (clinic_id, created_at DESC);
CREATE INDEX activity_recent_idx ON activity_log (created_at DESC);

CREATE TABLE payments (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id    uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    amount_cents integer NOT NULL CHECK (amount_cents >= 0),
    plan         text NOT NULL,
    months       integer NOT NULL CHECK (months BETWEEN 1 AND 36),
    note         text NOT NULL DEFAULT '',
    period_end   timestamptz NOT NULL,
    created_by   uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX payments_clinic_idx ON payments (clinic_id, created_at DESC);
