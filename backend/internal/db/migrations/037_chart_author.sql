-- Who signed a chart (id), so a printed plan can show the professional's title, cédula and contact.
ALTER TABLE patient_charts ADD COLUMN created_by uuid REFERENCES users (id) ON DELETE SET NULL;
