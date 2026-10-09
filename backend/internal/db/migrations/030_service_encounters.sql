-- Records of grooming, walks, training, boarding (animals) and therapy sessions are logbook entries too.
ALTER TABLE encounters DROP CONSTRAINT IF EXISTS encounters_kind_check;
ALTER TABLE encounters ADD CONSTRAINT encounters_kind_check CHECK (kind IN
    ('consulta', 'seguimiento', 'procedimiento', 'llamada', 'nota', 'adenda', 'estetica', 'paseo', 'adiestramiento', 'hospedaje', 'sesion'));
