-- Suscripción recurrente de Mercado Pago (preapproval): MP cobra solo cada mes o cada año.
-- Un checkout puede ser un pago único (como antes) o el alta de una suscripción.
ALTER TABLE billing_checkouts
    ADD COLUMN kind              text NOT NULL DEFAULT 'payment' CHECK (kind IN ('payment', 'subscription')),
    ADD COLUMN preapproval_id    text,
    ADD COLUMN preapproval_status text NOT NULL DEFAULT '';
CREATE UNIQUE INDEX billing_checkouts_preapproval_key ON billing_checkouts (preapproval_id) WHERE preapproval_id IS NOT NULL;

-- La suscripción vigente de la clínica. Al cancelar se conserva el acceso hasta el fin del periodo pagado.
ALTER TABLE clinics
    ADD COLUMN mp_preapproval_id   text NOT NULL DEFAULT '',
    ADD COLUMN cancel_at_period_end boolean NOT NULL DEFAULT false;
