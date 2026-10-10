-- Growth references that ship with Caresia (WHO and CDC tables): imports without a clinic. Every clinic sees them; a clinic's own
-- import of the same standard (a newer table it loaded) takes precedence for the indicators it covers.
ALTER TABLE growth_imports    ALTER COLUMN clinic_id DROP NOT NULL;
ALTER TABLE growth_references ALTER COLUMN clinic_id DROP NOT NULL;
CREATE UNIQUE INDEX growth_imports_platform_uq ON growth_imports (standard, version) WHERE clinic_id IS NULL;
