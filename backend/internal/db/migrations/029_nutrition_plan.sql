-- Nutrition plans are saved like the other versioned charts (each save is a new row).
ALTER TABLE patient_charts DROP CONSTRAINT IF EXISTS patient_charts_kind_check;
ALTER TABLE patient_charts ADD CONSTRAINT patient_charts_kind_check CHECK (kind IN ('odontogram', 'bodymap', 'nutrition_plan'));
