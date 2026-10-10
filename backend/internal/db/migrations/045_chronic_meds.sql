-- Chronic medication: what the patient takes every day, apart from the recetas of the clinic. Used to warn about
-- interactions and allergies when a new receta is written, and to remind about renewals.
CREATE TABLE chronic_medications (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id       uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    patient_id      uuid NOT NULL REFERENCES patients (id) ON DELETE CASCADE,
    name            text NOT NULL,
    dose            text NOT NULL DEFAULT '',
    frequency       text NOT NULL DEFAULT '',
    indication      text NOT NULL DEFAULT '',
    started_on      date,
    next_renewal    date,
    active          boolean NOT NULL DEFAULT true,
    stopped_at      timestamptz,
    stopped_reason  text NOT NULL DEFAULT '',
    created_by_name text NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX chronic_medications_patient_idx ON chronic_medications (clinic_id, patient_id, active);
CREATE INDEX chronic_medications_renewal_idx ON chronic_medications (clinic_id, next_renewal) WHERE active;

-- The reason a prescriber went on despite a severe interaction.
ALTER TABLE prescriptions ADD COLUMN interaction_override_reason text NOT NULL DEFAULT '';
