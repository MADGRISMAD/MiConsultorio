-- A branch runs only while the matrix's license allows branches. When the plan of the matrix stops including them,
-- its branches are taken out of service on their own (branch_suspended_by = 'license') and come back on their own when
-- a plan with branches returns. A branch the owner switched off ('owner') is never brought back by the license.
ALTER TABLE clinics ADD COLUMN branch_suspended_by text CHECK (branch_suspended_by IN ('owner', 'license'));
UPDATE clinics SET branch_suspended_by = 'owner' WHERE branch_suspended_at IS NOT NULL;

-- Mirrors Plan.MaxBranches > 1 of the Go catalog (a test keeps both in step).
CREATE FUNCTION plan_allows_branches(p text) RETURNS boolean AS $$
    SELECT p = 'pro'
$$ LANGUAGE sql IMMUTABLE;

CREATE OR REPLACE FUNCTION org_matrix_billing_to_branches() RETURNS trigger AS $$
BEGIN
    IF NEW.organization_id IS NOT NULL AND EXISTS (SELECT 1 FROM organizations o WHERE o.matrix_clinic_id = NEW.id) THEN
        UPDATE clinics SET plan = NEW.plan, billing_status = NEW.billing_status, trial_ends_at = NEW.trial_ends_at,
               current_period_end = NEW.current_period_end, suspended_at = NEW.suspended_at, suspended_reason = NEW.suspended_reason,
               branch_suspended_at = CASE
                   WHEN NOT plan_allows_branches(NEW.plan) THEN coalesce(branch_suspended_at, now())
                   WHEN branch_suspended_by = 'license' THEN NULL
                   ELSE branch_suspended_at END,
               branch_suspended_by = CASE
                   WHEN NOT plan_allows_branches(NEW.plan) THEN coalesce(branch_suspended_by, 'license')
                   WHEN branch_suspended_by = 'license' THEN NULL
                   ELSE branch_suspended_by END
         WHERE organization_id = NEW.organization_id AND id <> NEW.id;
    END IF;
    RETURN NULL;
END $$ LANGUAGE plpgsql;

-- Branches that already hang from a matrix whose plan does not include them.
UPDATE clinics b SET branch_suspended_at = now(), branch_suspended_by = 'license'
  FROM organizations o JOIN clinics m ON m.id = o.matrix_clinic_id
 WHERE b.organization_id = o.id AND b.id <> m.id AND NOT plan_allows_branches(m.plan) AND b.branch_suspended_at IS NULL;
