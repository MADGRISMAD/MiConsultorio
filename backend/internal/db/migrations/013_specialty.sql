-- Specialty record: vaccination card, odontogram and body-map snapshots, multi-session treatment plans
-- and digital informed consent. Clinical rows are append-only: they are voided with a reason or superseded
-- by a new snapshot, never edited or deleted (NOM-004-SSA3-2012).

-- Vaccination and deworming card, for people and animals. The patient portal reads these columns.
CREATE TABLE vaccinations (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id            uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    patient_id           uuid NOT NULL REFERENCES patients (id) ON DELETE CASCADE,
    kind                 text NOT NULL CHECK (kind IN ('vaccine', 'deworming_internal', 'deworming_external', 'other')),
    name                 text NOT NULL,
    applied_on           date NOT NULL,
    next_due             date,
    lot                  text NOT NULL DEFAULT '',
    dose                 text NOT NULL DEFAULT '',
    administered_by_name text NOT NULL DEFAULT '',
    notes                text NOT NULL DEFAULT '',
    catalog_item_id      uuid REFERENCES catalog_items (id) ON DELETE SET NULL,
    voided_at            timestamptz,
    voided_by_name       text NOT NULL DEFAULT '',
    void_reason          text NOT NULL DEFAULT '',
    created_by_name      text NOT NULL DEFAULT '',
    created_at           timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX vaccinations_patient_idx ON vaccinations (patient_id, applied_on DESC);
CREATE INDEX vaccinations_due_idx ON vaccinations (clinic_id, next_due) WHERE voided_at IS NULL AND next_due IS NOT NULL;

-- Odontogram and body-map snapshots: each save is a new row, the history is the list.
CREATE TABLE patient_charts (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id       uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    patient_id      uuid NOT NULL REFERENCES patients (id) ON DELETE CASCADE,
    kind            text NOT NULL CHECK (kind IN ('odontogram', 'bodymap')),
    data            jsonb NOT NULL,
    note            text NOT NULL DEFAULT '',
    encounter_id    uuid REFERENCES encounters (id) ON DELETE SET NULL,
    created_by_name text NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX patient_charts_patient_idx ON patient_charts (patient_id, kind, created_at DESC);

-- Treatment plans by phases. Once accepted, items are only added (new version) or cancelled, never removed.
CREATE TABLE treatment_plans (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id        uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    patient_id       uuid NOT NULL REFERENCES patients (id) ON DELETE CASCADE,
    title            text NOT NULL,
    status           text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'proposed', 'accepted', 'in_progress', 'completed', 'cancelled')),
    notes            text NOT NULL DEFAULT '',
    professional_id  uuid REFERENCES users (id) ON DELETE SET NULL,
    version          integer NOT NULL DEFAULT 1,
    accepted_version integer NOT NULL DEFAULT 0,
    accepted_at      timestamptz,
    accepted_by_name text NOT NULL DEFAULT '',
    cancel_reason    text NOT NULL DEFAULT '',
    created_by_name  text NOT NULL DEFAULT '',
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX treatment_plans_patient_idx ON treatment_plans (patient_id, created_at DESC);

CREATE TABLE treatment_plan_items (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id           uuid NOT NULL REFERENCES treatment_plans (id) ON DELETE CASCADE,
    clinic_id         uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    phase             integer NOT NULL DEFAULT 1 CHECK (phase BETWEEN 1 AND 50),
    position          integer NOT NULL DEFAULT 0,
    description       text NOT NULL,
    tooth             text NOT NULL DEFAULT '',
    catalog_item_id   uuid REFERENCES catalog_items (id) ON DELETE SET NULL,
    qty               numeric(12, 3) NOT NULL DEFAULT 1 CHECK (qty > 0),
    unit_price_cents  integer NOT NULL DEFAULT 0 CHECK (unit_price_cents >= 0),
    tax_rate          numeric(5, 2) NOT NULL DEFAULT 0 CHECK (tax_rate >= 0 AND tax_rate <= 100),
    status            text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'done', 'cancelled')),
    version_added     integer NOT NULL DEFAULT 1,
    done_at           timestamptz,
    done_by_name      text NOT NULL DEFAULT '',
    done_encounter_id uuid REFERENCES encounters (id) ON DELETE SET NULL,
    cancel_reason     text NOT NULL DEFAULT '',
    sale_id           uuid REFERENCES sales (id) ON DELETE SET NULL
);
CREATE INDEX treatment_plan_items_plan_idx ON treatment_plan_items (plan_id, phase, position);

CREATE TABLE treatment_plan_events (
    id         bigserial PRIMARY KEY,
    plan_id    uuid NOT NULL REFERENCES treatment_plans (id) ON DELETE CASCADE,
    clinic_id  uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    version    integer NOT NULL,
    action     text NOT NULL,
    detail     text NOT NULL DEFAULT '',
    actor_name text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX treatment_plan_events_plan_idx ON treatment_plan_events (plan_id, id);

-- Signed consent: the exact text shown, the drawn signature and a hash that ties them together.
CREATE TABLE consent_signatures (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id          uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    patient_id         uuid NOT NULL REFERENCES patients (id) ON DELETE CASCADE,
    plan_id            uuid REFERENCES treatment_plans (id) ON DELETE SET NULL,
    kind               text NOT NULL CHECK (kind IN ('procedimiento', 'plan_tratamiento', 'privacidad', 'telemedicina', 'animal', 'psicologia')),
    text_snapshot      text NOT NULL,
    signer_name        text NOT NULL,
    signer_role        text NOT NULL CHECK (signer_role IN ('paciente', 'tutor', 'propietario')),
    signature_png      text NOT NULL,
    witness1           text NOT NULL DEFAULT '',
    witness2           text NOT NULL DEFAULT '',
    content_sha256     text NOT NULL,
    signed_at          timestamptz NOT NULL DEFAULT now(),
    ip                 text NOT NULL DEFAULT '',
    user_agent         text NOT NULL DEFAULT '',
    registered_by_name text NOT NULL DEFAULT ''
);
CREATE INDEX consent_signatures_patient_idx ON consent_signatures (patient_id, signed_at DESC);

-- Append-only guards. Only UPDATE is blocked: rows leave with their patient or clinic through the foreign keys,
-- and the API has no delete route for any of these tables.
CREATE FUNCTION specialty_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION '% is append-only', TG_TABLE_NAME;
END $$;
CREATE TRIGGER patient_charts_append_only BEFORE UPDATE ON patient_charts FOR EACH ROW EXECUTE FUNCTION specialty_append_only();
CREATE TRIGGER consent_signatures_append_only BEFORE UPDATE ON consent_signatures FOR EACH ROW EXECUTE FUNCTION specialty_append_only();

-- A vaccination can only be voided: nothing but the void columns may change, and a voided row stays voided.
CREATE FUNCTION vaccinations_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.voided_at IS NOT NULL
       OR (NEW.id, NEW.clinic_id, NEW.patient_id, NEW.kind, NEW.name, NEW.applied_on, NEW.next_due, NEW.lot, NEW.dose,
           NEW.administered_by_name, NEW.notes, NEW.catalog_item_id, NEW.created_by_name, NEW.created_at)
          IS DISTINCT FROM
          (OLD.id, OLD.clinic_id, OLD.patient_id, OLD.kind, OLD.name, OLD.applied_on, OLD.next_due, OLD.lot, OLD.dose,
           OLD.administered_by_name, OLD.notes, OLD.catalog_item_id, OLD.created_by_name, OLD.created_at) THEN
        RAISE EXCEPTION 'vaccinations are append-only; void them instead';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER vaccinations_guard BEFORE UPDATE ON vaccinations FOR EACH ROW EXECUTE FUNCTION vaccinations_guard();
