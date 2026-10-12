-- Plan de pagos de un presupuesto: enganche y mensualidades (down_pct, installments, interval_days, start_date).
-- Es el acuerdo que se muestra e imprime; los cobros se registran en caja como abonos.
ALTER TABLE treatment_plans ADD COLUMN payment_plan jsonb;
