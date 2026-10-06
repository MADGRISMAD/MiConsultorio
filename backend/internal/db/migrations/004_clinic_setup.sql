-- More business types, extra specialties, scheduling settings and the first-run setup wizard state.
ALTER TABLE clinics DROP CONSTRAINT clinics_kind_check;
ALTER TABLE clinics ADD CONSTRAINT clinics_kind_check CHECK (kind IN (
    'GENERAL_MEDICAL', 'DENTAL', 'PEDIATRICS', 'INTERNAL_MEDICINE', 'PHYSIOTHERAPY', 'NUTRITION',
    'PSYCHOLOGY', 'DERMATOLOGY', 'GYNECOLOGY', 'ORTHOPEDICS', 'VETERINARY', 'CHIROPRACTIC'));

ALTER TABLE clinics
    ADD COLUMN specialties        text[] NOT NULL DEFAULT '{}',
    ADD COLUMN settings           jsonb NOT NULL DEFAULT '{}',
    ADD COLUMN setup_completed_at timestamptz;

ALTER TABLE clinics ADD CONSTRAINT clinics_specialties_check CHECK (specialties <@ ARRAY[
    'GENERAL_MEDICAL', 'DENTAL', 'PEDIATRICS', 'INTERNAL_MEDICINE', 'PHYSIOTHERAPY', 'NUTRITION',
    'PSYCHOLOGY', 'DERMATOLOGY', 'GYNECOLOGY', 'ORTHOPEDICS', 'VETERINARY', 'CHIROPRACTIC']::text[]);

-- Clinics that existed before the wizard are already set up.
UPDATE clinics SET setup_completed_at = now();
