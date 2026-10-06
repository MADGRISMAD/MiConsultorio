-- Business type ("giro") of each clinic.
ALTER TABLE clinics
    ADD COLUMN kind text NOT NULL DEFAULT 'GENERAL_MEDICAL'
    CHECK (kind IN ('GENERAL_MEDICAL', 'DENTAL', 'VETERINARY', 'CHIROPRACTIC'));
