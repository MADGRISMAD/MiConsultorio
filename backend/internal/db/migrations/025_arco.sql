-- ARCO requests (LFPDPPP): acceso, rectificación, cancelación, oposición and revocación of consent.
-- Requests arrive from staff or from a public form; every step is kept in an append-only history.

-- Short public id of the clinic, used in the public ARCO link when the clinic has no booking slug.
ALTER TABLE clinics ADD COLUMN arco_code text NOT NULL DEFAULT substr(md5(random()::text || clock_timestamp()::text), 1, 10);
CREATE UNIQUE INDEX clinics_arco_code_key ON clinics (arco_code);

CREATE TABLE arco_requests (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id            uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    seq                  integer NOT NULL,                 -- per-clinic counter
    folio                text NOT NULL,                    -- ARCO-2026-0001
    kind                 text NOT NULL CHECK (kind IN ('acceso', 'rectificacion', 'cancelacion', 'oposicion', 'revocacion')),
    patient_id           uuid REFERENCES patients (id),
    requester_name       text NOT NULL,
    requester_email      text NOT NULL DEFAULT '',
    requester_phone      text NOT NULL DEFAULT '',
    identity_verified    boolean NOT NULL DEFAULT false,
    identity_method      text NOT NULL DEFAULT '',         -- how the identity was checked (ID shown, signed letter, ...)
    identity_verified_at timestamptz,
    description          text NOT NULL,
    status               text NOT NULL DEFAULT 'recibida' CHECK (status IN ('recibida', 'en_revision', 'requiere_info', 'atendida', 'negada', 'vencida')),
    resolution           text NOT NULL DEFAULT '' CHECK (resolution IN ('', 'procedente', 'improcedente')),
    received_at          timestamptz NOT NULL DEFAULT now(),
    due_ack_at           date NOT NULL,                    -- internal target to acknowledge receipt
    due_answer_at        date NOT NULL,                    -- 20 business days to communicate the determination
    due_execute_at       date,                             -- 15 business days after a procedente determination
    answered_at          timestamptz,
    executed_at          timestamptz,
    response_text        text NOT NULL DEFAULT '',
    denial_reason        text NOT NULL DEFAULT '',
    handled_by_name      text NOT NULL DEFAULT '',
    created_via          text NOT NULL DEFAULT 'staff' CHECK (created_via IN ('staff', 'public')),
    token_hash           bytea NOT NULL,                   -- sha256 of the follow-up token given to the requester
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    UNIQUE (clinic_id, seq),
    UNIQUE (clinic_id, folio)
);
CREATE UNIQUE INDEX arco_requests_token_key ON arco_requests (token_hash);
CREATE INDEX arco_requests_clinic_status_idx ON arco_requests (clinic_id, status, due_answer_at);
CREATE INDEX arco_requests_patient_idx ON arco_requests (clinic_id, patient_id) WHERE patient_id IS NOT NULL;

CREATE TABLE arco_events (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id   uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    request_id  uuid NOT NULL REFERENCES arco_requests (id) ON DELETE CASCADE,
    kind        text NOT NULL,                             -- received | status | note | identity | link | response | executed | package | archive
    actor_name  text NOT NULL DEFAULT '',
    message     text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX arco_events_request_idx ON arco_events (clinic_id, request_id, created_at);

CREATE FUNCTION arco_events_append_only() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'arco_events is append-only';
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER arco_events_append_only BEFORE UPDATE ON arco_events FOR EACH ROW EXECUTE FUNCTION arco_events_append_only();
