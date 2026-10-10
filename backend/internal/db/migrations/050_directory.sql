-- Directorio público (como Doctoralia): los pacientes buscan por especialidad y ciudad, ven el perfil con
-- servicios y precios, opiniones verificadas y el próximo horario libre, y agendan.
ALTER TABLE clinic_profile
    ADD COLUMN listed          boolean NOT NULL DEFAULT false, -- aparece en el directorio (además de tener su página)
    ADD COLUMN city            text NOT NULL DEFAULT '',
    ADD COLUMN state           text NOT NULL DEFAULT '',
    ADD COLUMN neighborhood    text NOT NULL DEFAULT '',
    ADD COLUMN insurances      text[] NOT NULL DEFAULT '{}',    -- aseguradoras aceptadas
    ADD COLUMN languages       text[] NOT NULL DEFAULT '{}',
    ADD COLUMN payment_methods text[] NOT NULL DEFAULT '{}';
CREATE INDEX clinic_profile_directory_idx ON clinic_profile (lower(state), lower(city)) WHERE enabled AND listed;

-- Servicios que se muestran con su precio en el perfil público.
ALTER TABLE catalog_items ADD COLUMN public boolean NOT NULL DEFAULT false;

-- Formación y experiencia del profesional, para su ficha pública.
ALTER TABLE users ADD COLUMN public_bio text NOT NULL DEFAULT '';

-- El consultorio puede responder en público a una opinión.
ALTER TABLE satisfaction_surveys
    ADD COLUMN reply      text NOT NULL DEFAULT '',
    ADD COLUMN replied_at timestamptz;
