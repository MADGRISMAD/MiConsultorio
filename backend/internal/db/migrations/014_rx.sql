-- Recetas: public verification token (QR), override reasons and the clinic's own medicines.

ALTER TABLE prescriptions
    ADD COLUMN verify_token text,
    ADD COLUMN weight_kg numeric(6,2),
    ADD COLUMN allergy_override_reason text NOT NULL DEFAULT '',
    ADD COLUMN dose_override_reason text NOT NULL DEFAULT '';

-- Existing recetas get an unguessable token too (two random UUIDs = 244 random bits, no extension needed).
UPDATE prescriptions SET verify_token = replace(gen_random_uuid()::text || gen_random_uuid()::text, '-', '');
ALTER TABLE prescriptions ALTER COLUMN verify_token SET NOT NULL;
CREATE UNIQUE INDEX prescriptions_verify_token_key ON prescriptions (verify_token);

-- Medicines the clinic adds to the reference catalog (own formulas, local brands, veterinary products).
CREATE TABLE clinic_medications (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id     uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    name          text NOT NULL,                       -- denominación genérica
    brand         text NOT NULL DEFAULT '',
    subject       text NOT NULL DEFAULT 'person' CHECK (subject IN ('person', 'animal')),
    species       text[] NOT NULL DEFAULT '{}',        -- animals only; empty = all
    category      text NOT NULL DEFAULT '',
    control       text NOT NULL DEFAULT 'No' CHECK (control IN ('No', 'Antibiótico', 'Fracción III')),
    route         text NOT NULL DEFAULT 'Oral',
    presentations text[] NOT NULL DEFAULT '{}',
    typical_dose  text NOT NULL DEFAULT '',
    mg_per_kg     numeric(8,3),                        -- single dose, for weight-based calculation
    max_mg_per_kg_day numeric(8,3),
    concentrations jsonb NOT NULL DEFAULT '[]',        -- [{"label":"250 mg/5 mL","mg_per_ml":50}]
    notes         text NOT NULL DEFAULT '',
    active        boolean NOT NULL DEFAULT true,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX clinic_medications_clinic_idx ON clinic_medications (clinic_id, active);
