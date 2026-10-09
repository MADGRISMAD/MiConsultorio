-- Recetas: the area (giro) that issued each one, and whether it complements an earlier receta.
-- A new receta replaces the patient's earlier ones of the same area, unless it is a complement.
ALTER TABLE prescriptions
    ADD COLUMN area text NOT NULL DEFAULT '',
    ADD COLUMN complementary boolean NOT NULL DEFAULT false;
UPDATE prescriptions p SET area = c.kind FROM clinics c WHERE c.id = p.clinic_id;
