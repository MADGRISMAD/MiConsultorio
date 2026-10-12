-- Paquetes de sesiones: a catalog service with package_sessions set is a package (sold at the POS like any service).
-- Selling it to a patient creates a patient_packages balance that appointments draw down.
ALTER TABLE catalog_items
    ADD COLUMN package_sessions   integer CHECK (package_sessions BETWEEN 2 AND 100),
    ADD COLUMN package_service_id uuid REFERENCES catalog_items (id) ON DELETE SET NULL, -- NULL: any service
    ADD COLUMN package_valid_days integer CHECK (package_valid_days BETWEEN 1 AND 1825);  -- NULL: does not expire

CREATE TABLE patient_packages (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id       uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    patient_id      uuid NOT NULL REFERENCES patients (id) ON DELETE CASCADE,
    catalog_item_id uuid REFERENCES catalog_items (id) ON DELETE SET NULL,
    name            text NOT NULL,
    service_id      uuid REFERENCES catalog_items (id) ON DELETE SET NULL,
    sessions_total  integer NOT NULL CHECK (sessions_total > 0),
    sessions_used   integer NOT NULL DEFAULT 0 CHECK (sessions_used >= 0),
    expires_on      date,
    sale_id         uuid REFERENCES sales (id) ON DELETE SET NULL,
    status          text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'void')),
    created_at      timestamptz NOT NULL DEFAULT now(),
    CHECK (sessions_used <= sessions_total)
);
CREATE INDEX patient_packages_patient_idx ON patient_packages (clinic_id, patient_id) WHERE status = 'active';

ALTER TABLE appointments ADD COLUMN package_id uuid REFERENCES patient_packages (id) ON DELETE SET NULL;
