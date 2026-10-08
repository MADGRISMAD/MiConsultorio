-- Every clinic starts with a "Consulta" service in its catalog. It is what an appointment is charged as when it
-- ends without a service of its own, so the register can receive the consultation straight away.
-- system_key marks it (the name can be changed or the price adjusted; it is never deleted).

ALTER TABLE catalog_items ADD COLUMN system_key text;
CREATE UNIQUE INDEX catalog_items_system_key ON catalog_items (clinic_id, system_key) WHERE system_key IS NOT NULL;

-- Clinics that already have a service called "Consulta" keep it and mark it.
UPDATE catalog_items ci SET system_key = 'consulta'
FROM (
    SELECT DISTINCT ON (clinic_id) id FROM catalog_items
    WHERE kind = 'service' AND lower(trim(name)) = 'consulta' AND active
    ORDER BY clinic_id, created_at
) pick
WHERE ci.id = pick.id;

-- The rest get one. The price is an example ($500): each clinic sets its own in "Servicios y precios".
INSERT INTO catalog_items (clinic_id, kind, name, category, price_cents, unit, system_key, duration_minutes)
SELECT c.id, 'service', 'Consulta', 'Consulta', 50000, 'consulta', 'consulta', 30
FROM clinics c
WHERE NOT EXISTS (SELECT 1 FROM catalog_items x WHERE x.clinic_id = c.id AND x.system_key = 'consulta');

-- Clinics created from now on (sign-up, branches, platform panel) get it too.
CREATE FUNCTION caresia_default_consulta() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO catalog_items (clinic_id, kind, name, category, price_cents, unit, system_key, duration_minutes)
    VALUES (NEW.id, 'service', 'Consulta', 'Consulta', 50000, 'consulta', 'consulta', 30)
    ON CONFLICT DO NOTHING;
    RETURN NEW;
END $$;
CREATE TRIGGER clinics_default_consulta AFTER INSERT ON clinics FOR EACH ROW EXECUTE FUNCTION caresia_default_consulta();
