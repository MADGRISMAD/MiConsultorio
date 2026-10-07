-- Organizations: several branches (each one an independent clinic) under one owner.
-- A branch IS a clinic: every table keeps isolating by clinic_id. The organization only
-- links clinics together so the owner can switch between them and read consolidated reports.

CREATE TABLE organizations (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name             text NOT NULL,
    owner_user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    matrix_clinic_id uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE, -- holds the shared plan and subscription
    created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX organizations_owner_key ON organizations (owner_user_id);
CREATE UNIQUE INDEX organizations_matrix_key ON organizations (matrix_clinic_id);

ALTER TABLE clinics
    ADD COLUMN organization_id     uuid REFERENCES organizations (id) ON DELETE SET NULL,
    ADD COLUMN branch_suspended_at timestamptz; -- baja lógica de la sucursal; los datos clínicos nunca se borran
CREATE INDEX clinics_org_idx ON clinics (organization_id) WHERE organization_id IS NOT NULL;

-- A branch's own administrator account, linked to the organization owner. It has no e-mail and a
-- random password nobody knows: the only way into it is POST /org/switch, which re-checks ownership.
ALTER TABLE users
    ADD COLUMN linked_owner_id uuid REFERENCES users (id) ON DELETE CASCADE;
ALTER TABLE users ADD CONSTRAINT users_linked_owner_check CHECK (linked_owner_id IS NULL OR (role = 'admin' AND clinic_id IS NOT NULL AND email IS NULL));
CREATE UNIQUE INDEX users_linked_owner_key ON users (clinic_id, linked_owner_id) WHERE linked_owner_id IS NOT NULL;

-- Branches share the matrix's plan and subscription: whatever writes the matrix's billing columns
-- (platform staff, Mercado Pago, manual payments) is copied to the branches, and a branch cannot
-- hold different values.
CREATE FUNCTION org_branch_billing_from_matrix() RETURNS trigger AS $$
BEGIN
    IF NEW.organization_id IS NULL OR EXISTS (SELECT 1 FROM organizations o WHERE o.matrix_clinic_id = NEW.id) THEN
        RETURN NEW;
    END IF;
    SELECT m.plan, m.billing_status, m.trial_ends_at, m.current_period_end, m.suspended_at, m.suspended_reason
      INTO NEW.plan, NEW.billing_status, NEW.trial_ends_at, NEW.current_period_end, NEW.suspended_at, NEW.suspended_reason
      FROM organizations o JOIN clinics m ON m.id = o.matrix_clinic_id
     WHERE o.id = NEW.organization_id;
    RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE TRIGGER clinics_branch_billing_before
    BEFORE INSERT OR UPDATE OF organization_id, plan, billing_status, trial_ends_at, current_period_end, suspended_at, suspended_reason
    ON clinics FOR EACH ROW EXECUTE FUNCTION org_branch_billing_from_matrix();

CREATE FUNCTION org_matrix_billing_to_branches() RETURNS trigger AS $$
BEGIN
    IF NEW.organization_id IS NOT NULL AND EXISTS (SELECT 1 FROM organizations o WHERE o.matrix_clinic_id = NEW.id) THEN
        UPDATE clinics SET plan = NEW.plan, billing_status = NEW.billing_status, trial_ends_at = NEW.trial_ends_at,
               current_period_end = NEW.current_period_end, suspended_at = NEW.suspended_at, suspended_reason = NEW.suspended_reason
         WHERE organization_id = NEW.organization_id AND id <> NEW.id;
    END IF;
    RETURN NULL;
END $$ LANGUAGE plpgsql;

CREATE TRIGGER clinics_matrix_billing_after
    AFTER UPDATE OF plan, billing_status, trial_ends_at, current_period_end, suspended_at, suspended_reason
    ON clinics FOR EACH ROW EXECUTE FUNCTION org_matrix_billing_to_branches();
