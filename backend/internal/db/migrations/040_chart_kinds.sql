-- More versioned charts: validated scales, therapy goals, certificates, prenatal control, periodontogram, orthodontic
-- visits, developmental milestones, food recall, exercise sheets and the problem list.
ALTER TABLE patient_charts DROP CONSTRAINT IF EXISTS patient_charts_kind_check;
ALTER TABLE patient_charts ADD CONSTRAINT patient_charts_kind_check CHECK (kind IN
  ('odontogram', 'bodymap', 'nutrition_plan', 'scale', 'therapy_plan', 'certificate', 'prenatal', 'periodontogram', 'ortho_visit',
   'milestones', 'food_recall', 'exercises', 'problems'));
