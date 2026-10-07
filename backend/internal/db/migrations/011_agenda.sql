-- Agenda v2: a professional, a status and a room per appointment; time blocks; online booking
-- and reminder settings; the reminder queue. Other modules (reminders, booking, portal, POS)
-- build on these columns.

ALTER TABLE appointments
    ADD COLUMN professional_id uuid REFERENCES users (id) ON DELETE SET NULL,
    ADD COLUMN status        text NOT NULL DEFAULT 'scheduled'
        CHECK (status IN ('scheduled', 'confirmed', 'arrived', 'in_progress', 'completed', 'no_show', 'cancelled')),
    ADD COLUMN room          text NOT NULL DEFAULT '',
    ADD COLUMN source        text NOT NULL DEFAULT 'staff' CHECK (source IN ('staff', 'online', 'portal')),
    ADD COLUMN service_id    uuid REFERENCES catalog_items (id) ON DELETE SET NULL,
    ADD COLUMN encounter_id  uuid REFERENCES encounters (id) ON DELETE SET NULL,
    ADD COLUMN sale_id       uuid REFERENCES sales (id) ON DELETE SET NULL,
    ADD COLUMN phone         text NOT NULL DEFAULT '',
    ADD COLUMN email         text NOT NULL DEFAULT '',
    ADD COLUMN arrived_at    timestamptz,
    ADD COLUMN started_at    timestamptz,
    ADD COLUMN finished_at   timestamptz,
    ADD COLUMN cancel_reason text NOT NULL DEFAULT '',
    ADD COLUMN confirm_token text; -- lets the patient confirm or cancel from a link in the reminder
CREATE INDEX appointments_professional_idx ON appointments (clinic_id, professional_id, date, start_hour);
CREATE UNIQUE INDEX appointments_confirm_token_key ON appointments (confirm_token) WHERE confirm_token IS NOT NULL;

-- Per-professional agenda settings. Professionals without a row use the clinic defaults.
CREATE TABLE professional_settings (
    user_id        uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    clinic_id      uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    bookable       boolean NOT NULL DEFAULT false, -- appears in online booking
    slot_minutes   integer NOT NULL DEFAULT 30 CHECK (slot_minutes BETWEEN 5 AND 480),
    hours          jsonb NOT NULL DEFAULT '{}',    -- {"mon":[["09:00","14:00"],["16:00","19:00"]], ...} empty = clinic hours
    color          text NOT NULL DEFAULT '',
    updated_at     timestamptz NOT NULL DEFAULT now()
);

-- Vacations, emergencies, equipment maintenance. professional_id NULL blocks the whole clinic.
CREATE TABLE time_blocks (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id       uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    professional_id uuid REFERENCES users (id) ON DELETE CASCADE,
    date_from       date NOT NULL,
    date_to         date NOT NULL,
    start_hour      time,           -- NULL = all day
    end_hour        time,
    reason          text NOT NULL DEFAULT '',
    created_by_name text NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now(),
    CHECK (date_to >= date_from),
    CHECK ((start_hour IS NULL) = (end_hour IS NULL))
);
CREATE INDEX time_blocks_clinic_idx ON time_blocks (clinic_id, date_from, date_to);

-- One row per clinic.
CREATE TABLE agenda_settings (
    clinic_id           uuid PRIMARY KEY REFERENCES clinics (id) ON DELETE CASCADE,
    slot_minutes        integer NOT NULL DEFAULT 30 CHECK (slot_minutes BETWEEN 5 AND 480),
    rooms               text[] NOT NULL DEFAULT '{}',
    -- online booking (public page /reservar/<slug>)
    booking_enabled     boolean NOT NULL DEFAULT false,
    booking_slug        text,
    booking_lead_hours  integer NOT NULL DEFAULT 2,
    booking_horizon_days integer NOT NULL DEFAULT 30,
    booking_message     text NOT NULL DEFAULT '',
    booking_requires_confirmation boolean NOT NULL DEFAULT false,
    -- reminders
    remind_email        boolean NOT NULL DEFAULT true,
    remind_whatsapp     boolean NOT NULL DEFAULT false,
    remind_hours        integer[] NOT NULL DEFAULT '{24,2}',
    reminder_template   text NOT NULL DEFAULT '',
    updated_at          timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX agenda_settings_slug_key ON agenda_settings (lower(booking_slug)) WHERE booking_slug IS NOT NULL;

-- The patient agrees to receive reminders (a secondary purpose under the LFPDPPP).
ALTER TABLE patients ADD COLUMN reminders_ok boolean NOT NULL DEFAULT false;

CREATE TABLE appointment_reminders (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id      uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    appointment_id uuid NOT NULL REFERENCES appointments (id) ON DELETE CASCADE,
    channel        text NOT NULL CHECK (channel IN ('email', 'whatsapp')),
    hours_before   integer NOT NULL,
    send_at        timestamptz NOT NULL,
    status         text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'failed', 'skipped')),
    attempts       integer NOT NULL DEFAULT 0,
    error          text NOT NULL DEFAULT '',
    sent_at        timestamptz,
    UNIQUE (appointment_id, channel, hours_before)
);
CREATE INDEX appointment_reminders_due_idx ON appointment_reminders (send_at) WHERE status = 'pending';
