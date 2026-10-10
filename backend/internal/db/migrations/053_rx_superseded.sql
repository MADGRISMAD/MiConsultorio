-- A receta replaced by a newer one is "vencida" (superseded), not "cancelada": only a receta someone cancels carries a cancellation.
ALTER TABLE prescriptions ADD COLUMN superseded_at timestamptz, ADD COLUMN superseded_by_folio integer;
-- the ones the system used to cancel when it replaced them
UPDATE prescriptions
   SET superseded_at = voided_at,
       superseded_by_folio = nullif(regexp_replace(void_reason, '^Reemplazada por la receta #', ''), '')::integer,
       voided_at = NULL, voided_by = '', void_reason = ''
 WHERE voided_by = 'Sistema' AND void_reason ~ '^Reemplazada por la receta #[0-9]+$';
