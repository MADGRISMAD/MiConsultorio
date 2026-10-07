-- Patients (people and animals) with a kind-specific profile, the consultation log (bitácora),
-- prescriptions and the professional/legal data Mexican rules ask for. Replaces "expedients".

ALTER TABLE users
    ADD COLUMN cedula             text NOT NULL DEFAULT '', -- cédula profesional
    ADD COLUMN cedula_institution text NOT NULL DEFAULT '', -- institución que expidió el título
    ADD COLUMN cedula_specialty   text NOT NULL DEFAULT '', -- cédula de especialidad (opcional)
    ADD COLUMN specialty_title    text NOT NULL DEFAULT ''; -- profesión / especialidad que se imprime en la receta

ALTER TABLE clinics
    ADD COLUMN legal       jsonb   NOT NULL DEFAULT '{}', -- responsable sanitario, aviso de funcionamiento, contacto de privacidad
    ADD COLUMN patient_seq integer NOT NULL DEFAULT 0,
    ADD COLUMN rx_seq      integer NOT NULL DEFAULT 0;

CREATE TABLE patients (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id     uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    file_number   integer NOT NULL,                       -- número de expediente, por consultorio
    subject       text NOT NULL DEFAULT 'person' CHECK (subject IN ('person', 'animal')),
    names         text NOT NULL,
    last_names    text NOT NULL DEFAULT '',
    sex           text NOT NULL DEFAULT '',
    birth_date    date,
    curp          text NOT NULL DEFAULT '',
    phone         text NOT NULL DEFAULT '',
    email         text NOT NULL DEFAULT '',
    address       text NOT NULL DEFAULT '',
    guardian_name     text NOT NULL DEFAULT '',            -- tutor, responsable o propietario
    guardian_relation text NOT NULL DEFAULT '',
    guardian_phone    text NOT NULL DEFAULT '',
    guardian_email    text NOT NULL DEFAULT '',
    profile       jsonb NOT NULL DEFAULT '{}',            -- antecedentes según el giro
    incomplete    boolean NOT NULL DEFAULT false,         -- alta rápida sin antecedentes
    privacy_notice_at timestamptz,                        -- aviso de privacidad entregado / aceptado
    privacy_notice_by text NOT NULL DEFAULT '',
    last_encounter_at timestamptz,
    archived_at   timestamptz,
    archived_by   text NOT NULL DEFAULT '',
    archive_reason text NOT NULL DEFAULT '',
    created_by    text NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (clinic_id, file_number)
);
CREATE UNIQUE INDEX patients_curp_key ON patients (clinic_id, curp) WHERE curp <> '';
CREATE INDEX patients_clinic_idx ON patients (clinic_id, archived_at, lower(names));

-- Bitácora: entries are never edited or deleted (NOM-004-SSA3-2012). A mistake is fixed with an addendum.
CREATE TABLE encounters (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id      uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    patient_id     uuid NOT NULL REFERENCES patients (id) ON DELETE CASCADE,
    kind           text NOT NULL DEFAULT 'consulta' CHECK (kind IN ('consulta', 'seguimiento', 'procedimiento', 'llamada', 'nota', 'adenda')),
    occurred_at    timestamptz NOT NULL DEFAULT now(),
    appointment_id uuid,
    reason         text NOT NULL DEFAULT '',   -- motivo de consulta
    subjective     text NOT NULL DEFAULT '',   -- lo que cuenta el paciente (interrogatorio)
    measures       jsonb NOT NULL DEFAULT '{}',-- signos vitales y mediciones según el giro
    exam           text NOT NULL DEFAULT '',   -- exploración
    assessment     text NOT NULL DEFAULT '',   -- diagnóstico o impresión clínica
    diagnosis_codes text[] NOT NULL DEFAULT '{}', -- CIE-10 (opcional)
    plan           text NOT NULL DEFAULT '',   -- tratamiento e indicaciones
    notes          text NOT NULL DEFAULT '',
    private        boolean NOT NULL DEFAULT false, -- solo la persona que lo escribió puede leerlo (p. ej. psicoterapia)
    addendum_of    uuid REFERENCES encounters (id),
    author_id      uuid REFERENCES users (id) ON DELETE SET NULL,
    author_name    text NOT NULL,
    author_role    text NOT NULL DEFAULT '',
    author_license text NOT NULL DEFAULT '',   -- cédula al momento de firmar
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX encounters_patient_idx ON encounters (patient_id, occurred_at DESC);

CREATE TABLE prescriptions (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id      uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    patient_id     uuid NOT NULL REFERENCES patients (id) ON DELETE CASCADE,
    encounter_id   uuid REFERENCES encounters (id),
    folio          integer NOT NULL,
    mode           text NOT NULL DEFAULT 'medication' CHECK (mode IN ('medication', 'instructions')),
    issued_at      timestamptz NOT NULL DEFAULT now(),
    valid_until    date,
    diagnosis      text NOT NULL DEFAULT '',
    items          jsonb NOT NULL DEFAULT '[]',
    instructions   text NOT NULL DEFAULT '',
    next_visit     date,
    author_id      uuid REFERENCES users (id) ON DELETE SET NULL,
    author_name    text NOT NULL,
    author_title   text NOT NULL DEFAULT '',
    author_license text NOT NULL DEFAULT '',
    author_institution text NOT NULL DEFAULT '',
    author_specialty_license text NOT NULL DEFAULT '',
    voided_at      timestamptz,
    voided_by      text NOT NULL DEFAULT '',
    void_reason    text NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (clinic_id, folio)
);
CREATE INDEX prescriptions_patient_idx ON prescriptions (patient_id, issued_at DESC);

-- Who opened, printed or exported a record (traceability for NOM-024-SSA3-2012 and ARCO requests).
CREATE TABLE record_access (
    id         bigserial PRIMARY KEY,
    clinic_id  uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    patient_id uuid NOT NULL REFERENCES patients (id) ON DELETE CASCADE,
    user_id    uuid REFERENCES users (id) ON DELETE SET NULL,
    user_name  text NOT NULL,
    action     text NOT NULL CHECK (action IN ('view', 'print', 'export')),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX record_access_patient_idx ON record_access (patient_id, created_at DESC);

-- ---- carry the old expedients over ------------------------------------------------------------
INSERT INTO patients (clinic_id, file_number, subject, names, last_names, sex, birth_date, curp, profile, created_by, created_at, updated_at)
SELECT e.clinic_id,
       row_number() OVER (PARTITION BY e.clinic_id ORDER BY e.created_at, e.id),
       'person', e.names, e.last_names, e.sex, e.date_of_birth, e.curp,
       jsonb_strip_nulls(jsonb_build_object(
           'occupation', nullif(e.occupation, ''),
           'chronic_conditions', to_jsonb(array_remove(ARRAY[
               CASE WHEN e.diabetes THEN 'Diabetes' END,
               CASE WHEN e.cardiopathies THEN 'Cardiopatía' END,
               CASE WHEN e.cancer THEN 'Cáncer' END], NULL)),
           'allergies_text', CASE WHEN e.allergies THEN 'Refiere alergias (sin detalle en el historial anterior)' END,
           'surgeries', CASE WHEN e.surgeries THEN 'Refiere cirugías (sin detalle en el historial anterior)' END,
           'tobacco', CASE WHEN e.tabaquism THEN 'Diario' ELSE 'No' END,
           'alcohol', CASE WHEN e.alcoholism THEN 'Frecuente' ELSE 'No' END,
           'legacy_notes', nullif(concat_ws('; ',
               CASE WHEN e.education <> '' THEN 'Escolaridad: ' || e.education END,
               CASE WHEN e.weight <> '' THEN 'Peso: ' || e.weight END,
               CASE WHEN e.height <> '' THEN 'Talla: ' || e.height END,
               CASE WHEN e.ethnicity <> '' THEN 'Etnia: ' || e.ethnicity END,
               CASE WHEN e.physical_activity <> '' THEN 'Actividad física: ' || e.physical_activity END,
               CASE WHEN e.hobbies <> '' THEN 'Pasatiempos: ' || e.hobbies END,
               CASE WHEN e.child <> '' THEN 'Hijos: ' || e.child END,
               CASE WHEN e.rheumatic_diseases THEN 'Enfermedades reumáticas' END,
               CASE WHEN e.fractures THEN 'Fracturas' END,
               CASE WHEN e.layed THEN 'Encamamiento' END,
               CASE WHEN e.contractures THEN 'Contracturas' END,
               CASE WHEN e.accidents THEN 'Accidentes' END,
               CASE WHEN e.transfusions THEN 'Transfusiones' END,
               CASE WHEN e.automedication THEN 'Automedicación' END,
               CASE WHEN e.drug_use THEN 'Uso de drogas' END,
               CASE WHEN e.pregnant THEN 'Embarazo' END), '')
       )),
       'Migración', e.created_at, e.updated_at
FROM expedients e;

UPDATE clinics c SET patient_seq = coalesce((SELECT max(file_number) FROM patients p WHERE p.clinic_id = c.id), 0);

ALTER TABLE appointments ADD COLUMN patient_id uuid REFERENCES patients (id) ON DELETE SET NULL;
UPDATE appointments a SET patient_id = p.id FROM patients p WHERE p.clinic_id = a.clinic_id AND p.curp = a.curp AND a.curp <> '';
CREATE INDEX appointments_patient_idx ON appointments (patient_id);

DROP TABLE expedients;
