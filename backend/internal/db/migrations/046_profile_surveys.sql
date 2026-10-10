-- Public profile of the clinic and the satisfaction survey sent after a consultation.

CREATE TABLE clinic_profile (
    clinic_id        uuid PRIMARY KEY REFERENCES clinics (id) ON DELETE CASCADE,
    enabled          boolean NOT NULL DEFAULT false,      -- the public page /clinica/<slug> is live
    tagline          text NOT NULL DEFAULT '',
    about            text NOT NULL DEFAULT '',
    hours_text       text NOT NULL DEFAULT '',
    whatsapp         text NOT NULL DEFAULT '',
    website          text NOT NULL DEFAULT '',
    maps_url         text NOT NULL DEFAULT '',            -- the business listing on Google Maps
    google_place_id  text NOT NULL DEFAULT '',            -- builds the "write a review" link
    show_reviews     boolean NOT NULL DEFAULT true,       -- shows the rating and the comments patients allowed
    survey_enabled   boolean NOT NULL DEFAULT false,
    survey_delay_hours integer NOT NULL DEFAULT 3 CHECK (survey_delay_hours BETWEEN 1 AND 72),
    maps_min_rating  smallint NOT NULL DEFAULT 4 CHECK (maps_min_rating BETWEEN 1 AND 5), -- from this rating the patient is invited to Google
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE satisfaction_surveys (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id       uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    appointment_id  uuid NOT NULL UNIQUE REFERENCES appointments (id) ON DELETE CASCADE,
    patient_id      uuid REFERENCES patients (id) ON DELETE SET NULL,
    professional_id uuid REFERENCES users (id) ON DELETE SET NULL,
    token           text NOT NULL UNIQUE,
    attempts        integer NOT NULL DEFAULT 0,
    claimed_at      timestamptz,
    sent_at         timestamptz,
    answered_at     timestamptz,
    rating          smallint CHECK (rating BETWEEN 1 AND 5),
    comment         text NOT NULL DEFAULT '',
    public_ok       boolean NOT NULL DEFAULT false,       -- the patient allows showing the comment on the public page
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX satisfaction_clinic_idx ON satisfaction_surveys (clinic_id, answered_at DESC);
CREATE INDEX satisfaction_patient_idx ON satisfaction_surveys (patient_id, created_at DESC);
