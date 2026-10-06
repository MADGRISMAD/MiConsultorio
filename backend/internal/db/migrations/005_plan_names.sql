-- Plans are now Básico, Crecimiento and Pro. Collections (cobros) come with Crecimiento and Pro.
ALTER TABLE clinics DROP CONSTRAINT IF EXISTS clinics_plan_check;
UPDATE clinics SET plan = CASE plan WHEN 'consultorio' THEN 'basico' WHEN 'clinica' THEN 'crecimiento' WHEN 'empresarial' THEN 'pro' ELSE plan END;
UPDATE payments SET plan = CASE plan WHEN 'consultorio' THEN 'basico' WHEN 'clinica' THEN 'crecimiento' WHEN 'empresarial' THEN 'pro' ELSE plan END;
ALTER TABLE clinics ALTER COLUMN plan SET DEFAULT 'basico';
ALTER TABLE clinics ADD CONSTRAINT clinics_plan_check CHECK (plan IN ('basico', 'crecimiento', 'pro'));
