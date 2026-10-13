-- Página pública: servicios destacados con descripción, preguntas frecuentes, motivos para elegir el consultorio,
-- seguros (acepta o no) y redes sociales.
ALTER TABLE catalog_items
    ADD COLUMN public_description text NOT NULL DEFAULT '',
    ADD COLUMN featured boolean NOT NULL DEFAULT false;

ALTER TABLE clinic_profile
    ADD COLUMN accepts_insurance boolean NOT NULL DEFAULT false,
    ADD COLUMN highlights text[] NOT NULL DEFAULT '{}',
    ADD COLUMN faqs jsonb NOT NULL DEFAULT '[]',
    ADD COLUMN facebook_url text NOT NULL DEFAULT '',
    ADD COLUMN instagram_url text NOT NULL DEFAULT '';

-- quien ya tenía aseguradoras escritas las acepta
UPDATE clinic_profile SET accepts_insurance = true WHERE cardinality(insurances) > 0;
