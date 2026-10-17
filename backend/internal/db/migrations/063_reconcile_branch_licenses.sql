-- Safety net of the branch license rule: brings every branch in line with the plan of its matrix, whatever happened to the
-- rows (a plan changed before the trigger existed, a restore, a manual fix). The server runs it at start and every few minutes.
-- Returns how many branches it changed.
CREATE FUNCTION reconcile_branch_licenses() RETURNS integer AS $$
DECLARE off_n integer; on_n integer;
BEGIN
    UPDATE clinics b SET branch_suspended_at = now(), branch_suspended_by = 'license', updated_at = now()
      FROM organizations o JOIN clinics m ON m.id = o.matrix_clinic_id
     WHERE b.organization_id = o.id AND b.id <> m.id AND NOT plan_allows_branches(m.plan) AND b.branch_suspended_at IS NULL;
    GET DIAGNOSTICS off_n = ROW_COUNT;
    UPDATE clinics b SET branch_suspended_at = NULL, branch_suspended_by = NULL, updated_at = now()
      FROM organizations o JOIN clinics m ON m.id = o.matrix_clinic_id
     WHERE b.organization_id = o.id AND b.id <> m.id AND plan_allows_branches(m.plan) AND b.branch_suspended_by = 'license';
    GET DIAGNOSTICS on_n = ROW_COUNT;
    RETURN off_n + on_n;
END $$ LANGUAGE plpgsql;

SELECT reconcile_branch_licenses();
