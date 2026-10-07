-- Structured laboratory results and growth-chart references.
-- Lab results are append-only: a correction is a new row that points at the one it replaces (supersedes_id).
-- Orders are never deleted by the application: they are cancelled with a reason. The guards below refuse edits;
-- rows only disappear through the cascade of deleting a whole clinic or patient, which the application never does.

CREATE TABLE lab_orders (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id       uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    patient_id      uuid NOT NULL REFERENCES patients (id) ON DELETE CASCADE,
    encounter_id    uuid REFERENCES encounters (id) ON DELETE SET NULL,
    title           text NOT NULL,
    status          text NOT NULL DEFAULT 'solicitado' CHECK (status IN ('solicitado', 'parcial', 'completo', 'cancelado')),
    ordered_at      timestamptz NOT NULL DEFAULT now(),
    ordered_by_name text NOT NULL DEFAULT '',
    lab_name        text NOT NULL DEFAULT '',
    notes           text NOT NULL DEFAULT '',                       -- sealed (see lab.go)
    attachment_id   uuid REFERENCES attachments (id) ON DELETE SET NULL,
    cancel_reason   text NOT NULL DEFAULT '',
    cancelled_at    timestamptz,
    cancelled_by    text NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX lab_orders_patient_idx ON lab_orders (clinic_id, patient_id, ordered_at DESC);

CREATE TABLE lab_results (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id       uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    patient_id      uuid NOT NULL REFERENCES patients (id) ON DELETE CASCADE,
    order_id        uuid NOT NULL REFERENCES lab_orders (id) ON DELETE CASCADE,
    panel           text NOT NULL DEFAULT '',
    analyte         text NOT NULL,
    value_num       numeric,
    value_text      text NOT NULL DEFAULT '',
    unit            text NOT NULL DEFAULT '',
    ref_low         numeric,
    ref_high        numeric,
    ref_source      text NOT NULL DEFAULT '' CHECK (ref_source IN ('', 'laboratorio', 'catalogo')),
    flag            text NOT NULL DEFAULT 'na' CHECK (flag IN ('normal', 'bajo', 'alto', 'critico', 'anormal', 'na')),
    resulted_at     timestamptz NOT NULL DEFAULT now(),
    notes           text NOT NULL DEFAULT '',                       -- sealed; on a correction it holds the reason
    supersedes_id   uuid REFERENCES lab_results (id),
    created_by_name text NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now(),
    CHECK (value_num IS NOT NULL OR value_text <> ''),
    CHECK (ref_low IS NULL OR ref_high IS NULL OR ref_low <= ref_high)
);
CREATE INDEX lab_results_order_idx ON lab_results (order_id, created_at);
CREATE INDEX lab_results_trend_idx ON lab_results (clinic_id, patient_id, lower(analyte), resulted_at);
-- A result can be corrected only once: the correction itself is what gets corrected next.
CREATE UNIQUE INDEX lab_results_supersedes_uidx ON lab_results (supersedes_id) WHERE supersedes_id IS NOT NULL;

-- Nothing but the sealed notes may change (the encryption tool rewrites them on key rotation); no code path edits them.
CREATE FUNCTION lab_results_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF (NEW.id, NEW.clinic_id, NEW.patient_id, NEW.order_id, NEW.panel, NEW.analyte, NEW.value_num, NEW.value_text, NEW.unit, NEW.ref_low,
        NEW.ref_high, NEW.ref_source, NEW.flag, NEW.resulted_at, NEW.supersedes_id, NEW.created_by_name, NEW.created_at)
       IS DISTINCT FROM
       (OLD.id, OLD.clinic_id, OLD.patient_id, OLD.order_id, OLD.panel, OLD.analyte, OLD.value_num, OLD.value_text, OLD.unit, OLD.ref_low,
        OLD.ref_high, OLD.ref_source, OLD.flag, OLD.resulted_at, OLD.supersedes_id, OLD.created_by_name, OLD.created_at) THEN
        RAISE EXCEPTION 'lab results are append-only; capture a correction instead';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER lab_results_no_update BEFORE UPDATE ON lab_results FOR EACH ROW EXECUTE FUNCTION lab_results_guard();

-- An order keeps its identity; a cancelled order stays cancelled (only its sealed notes may be rewritten).
CREATE FUNCTION lab_orders_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF (NEW.id, NEW.clinic_id, NEW.patient_id, NEW.created_at) IS DISTINCT FROM (OLD.id, OLD.clinic_id, OLD.patient_id, OLD.created_at) THEN
        RAISE EXCEPTION 'lab order identity cannot change';
    END IF;
    IF OLD.status = 'cancelado' AND (NEW.title, NEW.status, NEW.ordered_at, NEW.lab_name, NEW.attachment_id, NEW.cancel_reason, NEW.cancelled_at)
       IS DISTINCT FROM (OLD.title, OLD.status, OLD.ordered_at, OLD.lab_name, OLD.attachment_id, OLD.cancel_reason, OLD.cancelled_at) THEN
        RAISE EXCEPTION 'a cancelled lab order cannot change';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER lab_orders_guard BEFORE UPDATE ON lab_orders FOR EACH ROW EXECUTE FUNCTION lab_orders_guard();

-- Growth references (WHO/CDC tables loaded by an administrator from a CSV; nothing is shipped in the product).
-- Each import is a new version of its standard; rows are never edited.
CREATE TABLE growth_imports (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id       uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    standard        text NOT NULL,
    version         integer NOT NULL,
    source_name     text NOT NULL,
    file_name       text NOT NULL DEFAULT '',
    sha256          text NOT NULL,
    row_count       integer NOT NULL,
    created_by_name text NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (clinic_id, standard, version)
);

CREATE TABLE growth_references (
    id         bigserial PRIMARY KEY,
    clinic_id  uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    import_id  uuid NOT NULL REFERENCES growth_imports (id) ON DELETE CASCADE,
    standard   text NOT NULL,
    indicator  text NOT NULL CHECK (indicator IN ('weight_for_age', 'length_height_for_age', 'bmi_for_age', 'head_circumference_for_age')),
    sex        text NOT NULL CHECK (sex IN ('M', 'F')),
    age_months numeric NOT NULL CHECK (age_months >= 0),
    l          numeric,
    m          numeric,
    s          numeric,
    pcts       jsonb NOT NULL DEFAULT '{}',   -- {"3": 9.1, "50": 12.2, ...}
    UNIQUE (import_id, indicator, sex, age_months)
);
CREATE INDEX growth_references_lookup_idx ON growth_references (clinic_id, standard, indicator, sex, age_months);

CREATE FUNCTION growth_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'growth references are append-only; import a new version';
END $$;
CREATE TRIGGER growth_references_guard BEFORE UPDATE ON growth_references FOR EACH ROW EXECUTE FUNCTION growth_append_only();
CREATE TRIGGER growth_imports_guard BEFORE UPDATE ON growth_imports FOR EACH ROW EXECUTE FUNCTION growth_append_only();
