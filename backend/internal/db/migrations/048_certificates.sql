-- Certificados médicos (personas) y veterinarios (animales). La incapacidad no se emite desde aquí.
ALTER TABLE clinics ADD COLUMN cert_seq integer NOT NULL DEFAULT 0;

CREATE TABLE certificates (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id     uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    patient_id    uuid NOT NULL REFERENCES patients (id) ON DELETE CASCADE,
    kind          text NOT NULL CHECK (kind IN ('medical', 'veterinary')),
    folio         integer NOT NULL,
    issued_at     timestamptz NOT NULL DEFAULT now(),
    valid_until   date,
    purpose       text NOT NULL DEFAULT '',
    statement     text NOT NULL DEFAULT '',   -- sealed: the text of the certificate
    findings      text NOT NULL DEFAULT '',   -- sealed: what was found on examination (optional)
    extra         jsonb NOT NULL DEFAULT '{}', -- aptitude, restrictions; vet: destination, microchip
    author_id     uuid REFERENCES users (id) ON DELETE SET NULL,
    author_name   text NOT NULL DEFAULT '',
    author_title  text NOT NULL DEFAULT '',
    author_license text NOT NULL DEFAULT '',
    author_institution text NOT NULL DEFAULT '',
    author_specialty_license text NOT NULL DEFAULT '',
    verify_token  text NOT NULL UNIQUE,
    voided_at     timestamptz,
    voided_by     text NOT NULL DEFAULT '',
    void_reason   text NOT NULL DEFAULT '',
    UNIQUE (clinic_id, folio)
);
CREATE INDEX certificates_patient_idx ON certificates (clinic_id, patient_id, issued_at DESC);
