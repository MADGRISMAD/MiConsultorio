-- Notices sent to the platform team about payments that did not arrive (a failed charge, a period that ended unpaid, a trial
-- that ended unpaid). One row per event, so a notice goes out once however many times it is detected.
CREATE TABLE billing_notices (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id  uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    kind       text NOT NULL,   -- payment_failed | period_ended | trial_ended
    ref        text NOT NULL,   -- what the event is about (the period or trial that ended, the subscription)
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (clinic_id, kind, ref)
);
