-- The plan a visitor asked to try with the "Solicitar demo" card they pressed. Every trial starts on Básico; this only
-- tells the platform team which plan to activate for them, and is shown in the platform panel until it is done.
ALTER TABLE clinics ADD COLUMN requested_plan text;
