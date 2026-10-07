-- Indexes for the operational and clinical reports (range scans per clinic).
CREATE INDEX encounters_clinic_time_idx ON encounters (clinic_id, occurred_at) WHERE addendum_of IS NULL;
CREATE INDEX appointments_clinic_status_idx ON appointments (clinic_id, date, status);
CREATE INDEX patients_clinic_created_idx ON patients (clinic_id, created_at);
CREATE INDEX patients_clinic_lastenc_idx ON patients (clinic_id, last_encounter_at) WHERE archived_at IS NULL;
