-- Attachments of a patient's record (radiografías, laboratorio, consentimientos, fotos). The bytes live
-- encrypted on disk under UPLOADS_DIR; this table keeps the metadata. Clinical files are never deleted:
-- they are archived with a reason (NOM-004-SSA3-2012, conservation of at least 5 years).

CREATE TABLE attachments (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id        uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    patient_id       uuid NOT NULL REFERENCES patients (id) ON DELETE CASCADE,
    encounter_id     uuid REFERENCES encounters (id) ON DELETE SET NULL,
    kind             text NOT NULL DEFAULT 'document' CHECK (kind IN ('xray', 'lab', 'ultrasound', 'consent', 'photo', 'document', 'other')),
    title            text NOT NULL DEFAULT '',
    note             text NOT NULL DEFAULT '',
    original_name    text NOT NULL DEFAULT '',
    mime             text NOT NULL,
    size_bytes       bigint NOT NULL CHECK (size_bytes >= 0), -- plaintext size
    sha256           text NOT NULL,                           -- of the plaintext, checked on every read
    storage_key      text NOT NULL UNIQUE,                    -- <clinic_id>/<aa>/<random uuid>, never a user-given name
    uploaded_by      uuid REFERENCES users (id) ON DELETE SET NULL,
    uploaded_by_name text NOT NULL DEFAULT '',
    created_at       timestamptz NOT NULL DEFAULT now(),
    archived_at      timestamptz,
    archived_by_name text NOT NULL DEFAULT '',
    archive_reason   text NOT NULL DEFAULT ''
);
CREATE INDEX attachments_patient_idx ON attachments (clinic_id, patient_id, created_at DESC);
CREATE INDEX attachments_clinic_idx ON attachments (clinic_id);

-- Uploading, opening, downloading and archiving files are recorded in the access log of the record.
ALTER TABLE record_access DROP CONSTRAINT record_access_action_check;
ALTER TABLE record_access ADD CONSTRAINT record_access_action_check
    CHECK (action IN ('view', 'print', 'export', 'file_upload', 'file_view', 'file_download', 'file_archive'));
