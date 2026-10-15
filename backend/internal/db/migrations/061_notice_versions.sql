-- Which version of the privacy notice / terms each person accepted, and when (the text itself lives in the pages).
ALTER TABLE appointments ADD COLUMN privacy_version text;
ALTER TABLE users ADD COLUMN terms_accepted_at timestamptz, ADD COLUMN terms_version text;
