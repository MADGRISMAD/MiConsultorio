-- Joining two records of the same person moves their rows to one patient. The append-only guards let a row change
-- patient (and nothing else) only inside a transaction that says it is a merge: set_config('app.patient_merge', 'on', true).
CREATE FUNCTION patient_merge_only(o jsonb, n jsonb) RETURNS boolean LANGUAGE sql STABLE AS $$
    SELECT coalesce(current_setting('app.patient_merge', true), '') = 'on' AND (o - 'patient_id') = (n - 'patient_id')
$$;

CREATE OR REPLACE FUNCTION specialty_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF patient_merge_only(to_jsonb(OLD), to_jsonb(NEW)) THEN RETURN NEW; END IF;
    RAISE EXCEPTION '% is append-only', TG_TABLE_NAME;
END $$;

CREATE OR REPLACE FUNCTION vaccinations_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF patient_merge_only(to_jsonb(OLD), to_jsonb(NEW)) THEN RETURN NEW; END IF;
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

CREATE OR REPLACE FUNCTION lab_results_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF patient_merge_only(to_jsonb(OLD), to_jsonb(NEW)) THEN RETURN NEW; END IF;
    IF (NEW.id, NEW.clinic_id, NEW.patient_id, NEW.order_id, NEW.panel, NEW.analyte, NEW.value_num, NEW.value_text, NEW.unit, NEW.ref_low,
        NEW.ref_high, NEW.ref_source, NEW.flag, NEW.resulted_at, NEW.supersedes_id, NEW.created_by_name, NEW.created_at)
       IS DISTINCT FROM
       (OLD.id, OLD.clinic_id, OLD.patient_id, OLD.order_id, OLD.panel, OLD.analyte, OLD.value_num, OLD.value_text, OLD.unit, OLD.ref_low,
        OLD.ref_high, OLD.ref_source, OLD.flag, OLD.resulted_at, OLD.supersedes_id, OLD.created_by_name, OLD.created_at) THEN
        RAISE EXCEPTION 'lab results are append-only; capture a correction instead';
    END IF;
    RETURN NEW;
END $$;

CREATE OR REPLACE FUNCTION lab_orders_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF patient_merge_only(to_jsonb(OLD), to_jsonb(NEW)) THEN RETURN NEW; END IF;
    IF (NEW.id, NEW.clinic_id, NEW.patient_id, NEW.created_at) IS DISTINCT FROM (OLD.id, OLD.clinic_id, OLD.patient_id, OLD.created_at) THEN
        RAISE EXCEPTION 'lab order identity cannot change';
    END IF;
    IF OLD.status = 'cancelado' AND (NEW.title, NEW.status, NEW.ordered_at, NEW.lab_name, NEW.attachment_id, NEW.cancel_reason, NEW.cancelled_at)
       IS DISTINCT FROM (OLD.title, OLD.status, OLD.ordered_at, OLD.lab_name, OLD.attachment_id, OLD.cancel_reason, OLD.cancelled_at) THEN
        RAISE EXCEPTION 'a cancelled lab order cannot change';
    END IF;
    RETURN NEW;
END $$;
