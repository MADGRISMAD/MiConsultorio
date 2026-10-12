-- Cobro anticipado: an online booking of a priced service can be held with a percentage paid up front
-- (Mercado Pago, the clinic's own account). The appointment waits as pending_payment until it is paid.
ALTER TABLE agenda_settings ADD COLUMN deposit_percent integer NOT NULL DEFAULT 0 CHECK (deposit_percent BETWEEN 0 AND 100);
-- NULL: the clinic's percentage applies
ALTER TABLE catalog_items ADD COLUMN deposit_percent integer CHECK (deposit_percent BETWEEN 0 AND 100);

ALTER TABLE appointments DROP CONSTRAINT appointments_status_check;
ALTER TABLE appointments ADD CONSTRAINT appointments_status_check
    CHECK (status IN ('pending_payment', 'scheduled', 'confirmed', 'arrived', 'in_progress', 'completed', 'no_show', 'cancelled'));
ALTER TABLE appointments
    ADD COLUMN deposit_cents      integer NOT NULL DEFAULT 0,
    ADD COLUMN deposit_state      text NOT NULL DEFAULT 'none' CHECK (deposit_state IN ('none', 'pending', 'paid', 'refunded', 'kept', 'applied')),
    ADD COLUMN deposit_charge_id  text,
    ADD COLUMN deposit_expires_at timestamptz;
CREATE INDEX appointments_deposit_due_idx ON appointments (deposit_expires_at) WHERE status = 'pending_payment';
