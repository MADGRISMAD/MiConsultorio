-- Birthday greetings: one row per person, pet or owner and year, so each greeting goes out once even with several API instances.
CREATE TABLE birthday_mail (
    kind       text NOT NULL CHECK (kind IN ('patient', 'owner')),
    ref_id     uuid NOT NULL,
    year       int  NOT NULL,
    clinic_id  uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    attempts   int  NOT NULL DEFAULT 0,
    claimed_at timestamptz NOT NULL DEFAULT now(),
    sent_at    timestamptz,
    PRIMARY KEY (kind, ref_id, year)
);
CREATE INDEX birthday_mail_clinic_idx ON birthday_mail (clinic_id);
