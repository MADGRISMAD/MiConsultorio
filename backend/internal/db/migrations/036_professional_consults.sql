-- An owner or administrator who does not see patients ("solo soy el dueño") is not offered for appointments.
ALTER TABLE professional_settings ADD COLUMN consults boolean NOT NULL DEFAULT true;
