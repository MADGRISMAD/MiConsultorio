-- Each patient belongs to the giro(s) the clinic had when they were registered. When the clinic changes giro, the
-- patients of the giros it no longer works with stop showing in lists and searches; nothing is deleted, and they
-- show again if the giro comes back. Empty = always visible.
ALTER TABLE patients ADD COLUMN kinds text[] NOT NULL DEFAULT '{}';

UPDATE patients p SET kinds = CASE
    WHEN p.subject = 'animal' THEN ARRAY['VETERINARY']
    ELSE coalesce(
        nullif(array(SELECT k FROM unnest(ARRAY[c.kind] || c.specialties) AS k WHERE k <> 'VETERINARY'), '{}'),
        ARRAY['GENERAL_MEDICAL'])
    END
FROM clinics c WHERE c.id = p.clinic_id;
