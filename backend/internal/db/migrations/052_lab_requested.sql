-- The studies a professional asks for on an order (the sheet the patient takes to a laboratory): names of panels or single analytes.
ALTER TABLE lab_orders ADD COLUMN requested text[] NOT NULL DEFAULT '{}';
