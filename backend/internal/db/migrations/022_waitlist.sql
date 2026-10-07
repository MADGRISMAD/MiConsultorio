-- Booking v2 and notifications: duration per service, the waitlist with its time-limited offers,
-- and the in-app notifications (bell) of the team.

-- A service may take longer (or less) than the professional's usual slot. NULL = use the professional's slot.
ALTER TABLE catalog_items
    ADD COLUMN duration_minutes integer CHECK (duration_minutes IS NULL OR duration_minutes BETWEEN 5 AND 480);

CREATE TABLE waitlist_entries (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id       uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    patient_id      uuid REFERENCES patients (id) ON DELETE SET NULL,
    name            text NOT NULL,
    phone           text NOT NULL DEFAULT '',
    email           text NOT NULL DEFAULT '',
    professional_id uuid REFERENCES users (id) ON DELETE SET NULL, -- NULL = any professional
    service_id      uuid REFERENCES catalog_items (id) ON DELETE SET NULL,
    days            smallint[] NOT NULL DEFAULT '{}',              -- preferred weekdays, 0 = Sunday ... 6 = Saturday; empty = any
    from_time       time,                                          -- preferred window of the day; NULL = any
    to_time         time,
    notes           text NOT NULL DEFAULT '',
    consent         boolean NOT NULL DEFAULT false,                -- agreed to be contacted about a free slot
    status          text NOT NULL DEFAULT 'waiting' CHECK (status IN ('waiting', 'offered', 'booked', 'expired', 'cancelled')),
    token           text NOT NULL UNIQUE,
    created_via     text NOT NULL DEFAULT 'staff' CHECK (created_via IN ('staff', 'online')),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CHECK ((from_time IS NULL) = (to_time IS NULL)),
    CHECK (to_time IS NULL OR to_time > from_time)
);
CREATE INDEX waitlist_entries_clinic_idx ON waitlist_entries (clinic_id, status, created_at);

CREATE TABLE waitlist_offers (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id       uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    entry_id        uuid NOT NULL REFERENCES waitlist_entries (id) ON DELETE CASCADE,
    professional_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    service_id      uuid REFERENCES catalog_items (id) ON DELETE SET NULL,
    date            date NOT NULL,
    start_hour      time NOT NULL,
    end_hour        time NOT NULL,
    status          text NOT NULL DEFAULT 'offered' CHECK (status IN ('offered', 'accepted', 'declined', 'expired', 'cancelled')),
    expires_at      timestamptz NOT NULL,
    appointment_id  uuid REFERENCES appointments (id) ON DELETE SET NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    responded_at    timestamptz,
    CHECK (end_hour > start_hour)
);
-- One live offer per entry and per slot start.
CREATE UNIQUE INDEX waitlist_offers_entry_live_key ON waitlist_offers (entry_id) WHERE status = 'offered';
CREATE UNIQUE INDEX waitlist_offers_slot_live_key ON waitlist_offers (clinic_id, professional_id, date, start_hour) WHERE status = 'offered';
CREATE INDEX waitlist_offers_due_idx ON waitlist_offers (expires_at) WHERE status = 'offered';
CREATE INDEX waitlist_offers_entry_idx ON waitlist_offers (entry_id);

-- In-app notifications. user_id NULL = for everyone whose role holds `perm` (empty perm = every team member);
-- those are read per person in notification_reads. A notification aimed at one user uses read_at.
CREATE TABLE notifications (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id  uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    user_id    uuid REFERENCES users (id) ON DELETE CASCADE,
    perm       text NOT NULL DEFAULT '',
    kind       text NOT NULL,
    title      text NOT NULL,
    body       text NOT NULL DEFAULT '',
    link       text NOT NULL DEFAULT '',
    dedupe_key text,
    read_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notifications_clinic_idx ON notifications (clinic_id, created_at DESC);
CREATE INDEX notifications_user_idx ON notifications (user_id, created_at DESC) WHERE user_id IS NOT NULL;
CREATE UNIQUE INDEX notifications_dedupe_key ON notifications (clinic_id, dedupe_key) WHERE dedupe_key IS NOT NULL;

CREATE TABLE notification_reads (
    notification_id uuid NOT NULL REFERENCES notifications (id) ON DELETE CASCADE,
    user_id         uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    read_at         timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (notification_id, user_id)
);
