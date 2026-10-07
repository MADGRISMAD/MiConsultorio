-- Mercado Pago: subscription checkouts, per-clinic Point/link charges, encrypted OAuth tokens; magic (AI) usage.

ALTER TABLE payments
    ADD COLUMN provider     text NOT NULL DEFAULT 'manual',
    ADD COLUMN provider_ref text;
CREATE UNIQUE INDEX payments_provider_ref_key ON payments (provider, provider_ref) WHERE provider_ref IS NOT NULL;

-- A request to pay for a plan period online. It turns into a payments row once Mercado Pago confirms.
CREATE TABLE billing_checkouts (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id     uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    plan          text NOT NULL CHECK (plan IN ('basico', 'crecimiento', 'pro')),
    period        text NOT NULL CHECK (period IN ('month', 'year')),
    amount_cents  integer NOT NULL CHECK (amount_cents > 0),
    status        text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'paid', 'failed', 'expired')),
    preference_id text NOT NULL DEFAULT '',
    init_point    text NOT NULL DEFAULT '',
    mp_payment_id text,
    created_by    uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    paid_at       timestamptz
);
CREATE INDEX billing_checkouts_clinic_idx ON billing_checkouts (clinic_id, created_at DESC);

-- Each clinic connects its own Mercado Pago account (OAuth) to charge its patients.
CREATE TABLE mp_accounts (
    clinic_id     uuid PRIMARY KEY REFERENCES clinics (id) ON DELETE CASCADE,
    mp_user_id    text NOT NULL,
    access_token  bytea NOT NULL, -- AES-GCM
    refresh_token bytea NOT NULL,
    expires_at    timestamptz,
    connected_by  text NOT NULL DEFAULT '',
    connected_at  timestamptz NOT NULL DEFAULT now()
);

-- A card-terminal (Point) payment intent or a payment link made from the POS.
CREATE TABLE mp_charges (
    id            text PRIMARY KEY, -- Mercado Pago intent id or preference id
    clinic_id     uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    kind          text NOT NULL CHECK (kind IN ('point', 'link')),
    device_id     text NOT NULL DEFAULT '',
    amount_cents  integer NOT NULL CHECK (amount_cents > 0),
    status        text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'approved', 'canceled', 'error')),
    external_ref  text NOT NULL DEFAULT '',
    pay_url       text NOT NULL DEFAULT '',
    mp_payment_id text,
    sale_id       uuid REFERENCES sales (id) ON DELETE SET NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX mp_charges_clinic_idx ON mp_charges (clinic_id, created_at DESC);

CREATE TABLE magic_usage (
    clinic_id uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    month     text NOT NULL, -- YYYY-MM
    used      integer NOT NULL DEFAULT 0,
    PRIMARY KEY (clinic_id, month)
);
