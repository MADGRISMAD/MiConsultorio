-- Owners (propietarios) of animal patients. Until now the owner was three text columns copied into every pet,
-- so two pets of the same person were not linked. An owner is now a record; its pets point to it.
-- patients.guardian_* keep a copy of the owner's data (printing, reminders and the portal read them).

-- Two owners are the same person when the name matches and so does the contact: the phone (last 10 digits)
-- or, without a phone, the e-mail. Same name with another phone is another person ("Max de Luis", "Max de Marco").
CREATE FUNCTION caresia_contact_key(phone text, email text) RETURNS text LANGUAGE sql IMMUTABLE AS $$
    SELECT coalesce(nullif(right(regexp_replace(coalesce(phone, ''), '\D', '', 'g'), 10), ''), lower(btrim(coalesce(email, ''))), '')
$$;

-- Names are compared without case and with single spaces.
CREATE FUNCTION caresia_name_key(name text) RETURNS text LANGUAGE sql IMMUTABLE AS $$
    SELECT regexp_replace(lower(btrim(coalesce(name, ''))), '\s+', ' ', 'g')
$$;

CREATE TABLE owners (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id  uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    name       text NOT NULL,
    phone      text NOT NULL DEFAULT '',
    email      text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX owners_clinic_name_idx ON owners (clinic_id, lower(name));

ALTER TABLE patients ADD COLUMN owner_id uuid REFERENCES owners (id) ON DELETE SET NULL;
CREATE INDEX patients_owner_idx ON patients (owner_id) WHERE owner_id IS NOT NULL;

-- Group the animals that already exist.
CREATE TEMP TABLE owner_groups AS
SELECT gen_random_uuid() AS id, clinic_id,
       caresia_name_key(guardian_name) AS nkey, caresia_contact_key(guardian_phone, guardian_email) AS ckey,
       regexp_replace((array_agg(btrim(guardian_name) ORDER BY created_at))[1], '\s+', ' ', 'g') AS name,
       coalesce((array_agg(guardian_phone ORDER BY (guardian_phone = ''), created_at))[1], '') AS phone,
       coalesce((array_agg(guardian_email ORDER BY (guardian_email = ''), created_at))[1], '') AS email
FROM patients
WHERE subject = 'animal' AND btrim(guardian_name) <> ''
GROUP BY clinic_id, caresia_name_key(guardian_name), caresia_contact_key(guardian_phone, guardian_email);

INSERT INTO owners (id, clinic_id, name, phone, email) SELECT id, clinic_id, name, phone, email FROM owner_groups;

UPDATE patients p SET owner_id = g.id
FROM owner_groups g
WHERE p.subject = 'animal' AND p.clinic_id = g.clinic_id
  AND caresia_name_key(p.guardian_name) = g.nkey AND caresia_contact_key(p.guardian_phone, p.guardian_email) = g.ckey;

DROP TABLE owner_groups;
