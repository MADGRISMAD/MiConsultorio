-- The date the professional suggests for the next consultation, so the receta can start from it.
ALTER TABLE encounters ADD COLUMN next_visit date;
