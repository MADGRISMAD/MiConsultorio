-- The owner's domicilio and birth date (informative only). Pets keep a copy of the domicilio in patients.address.
ALTER TABLE owners ADD COLUMN address text NOT NULL DEFAULT '', ADD COLUMN birth_date date;
