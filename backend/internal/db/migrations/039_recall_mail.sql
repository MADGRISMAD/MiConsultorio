-- Follow-up reminders by e-mail (vaccines due, suggested next visit, dental check-up, Papanicolaou): each one is sent once.
CREATE TABLE recall_mail (
    kind       text NOT NULL,
    ref        text NOT NULL,
    due        date NOT NULL,
    clinic_id  uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    attempts   int  NOT NULL DEFAULT 0,
    claimed_at timestamptz NOT NULL DEFAULT now(),
    sent_at    timestamptz,
    PRIMARY KEY (kind, ref, due)
);
CREATE INDEX recall_mail_clinic_idx ON recall_mail (clinic_id);
