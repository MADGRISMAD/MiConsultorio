-- A patient who picks a time on the public booking page holds it for a few minutes, so two people do not fill in the form for the same slot.
CREATE TABLE booking_holds (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id       uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    professional_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    date            date NOT NULL,
    start_hour      time NOT NULL,
    end_hour        time NOT NULL,
    holder          text NOT NULL,
    expires_at      timestamptz NOT NULL
);
CREATE INDEX booking_holds_slot_idx ON booking_holds (clinic_id, professional_id, date);
CREATE INDEX booking_holds_exp_idx ON booking_holds (expires_at);
