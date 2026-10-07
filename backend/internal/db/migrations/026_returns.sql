-- Devoluciones: a customer brings back some of what a sale included. The sale stays as it was
-- (paid); each return is its own record that gives stock back, refunds money and reverses commission.

ALTER TABLE clinics ADD COLUMN return_seq integer NOT NULL DEFAULT 0;

CREATE TABLE sale_returns (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id           uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    sale_id             uuid NOT NULL REFERENCES sales (id) ON DELETE CASCADE,
    folio               integer NOT NULL,
    reason              text NOT NULL,
    total_cents         integer NOT NULL CHECK (total_cents > 0),
    credit_note_pending boolean NOT NULL DEFAULT false, -- the sale had an issued CFDI: an egreso note is owed
    session_id          uuid REFERENCES cash_sessions (id) ON DELETE SET NULL,
    created_by          uuid REFERENCES users (id) ON DELETE SET NULL,
    created_by_name     text NOT NULL DEFAULT '',
    created_at          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (clinic_id, folio)
);
CREATE INDEX sale_returns_sale_idx ON sale_returns (sale_id);
CREATE INDEX sale_returns_clinic_date_idx ON sale_returns (clinic_id, created_at DESC);

CREATE TABLE sale_return_items (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    return_id        uuid NOT NULL REFERENCES sale_returns (id) ON DELETE CASCADE,
    sale_item_id     uuid NOT NULL REFERENCES sale_items (id) ON DELETE CASCADE,
    item_id          uuid REFERENCES catalog_items (id) ON DELETE SET NULL,
    professional_id  uuid REFERENCES users (id) ON DELETE SET NULL,
    name             text NOT NULL,
    qty              numeric(12, 3) NOT NULL CHECK (qty > 0),
    amount_cents     integer NOT NULL CHECK (amount_cents >= 0),
    commission_cents integer NOT NULL DEFAULT 0,
    commission_base_cents integer NOT NULL DEFAULT 0,
    restocked        boolean NOT NULL DEFAULT false
);
CREATE INDEX sale_return_items_sale_item_idx ON sale_return_items (sale_item_id);

CREATE TABLE sale_return_refunds (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    return_id    uuid NOT NULL REFERENCES sale_returns (id) ON DELETE CASCADE,
    method       text NOT NULL CHECK (method IN ('cash', 'card', 'transfer', 'mp_point', 'mp_link', 'other')),
    amount_cents integer NOT NULL CHECK (amount_cents > 0),
    reference    text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sale_return_refunds_return_idx ON sale_return_refunds (return_id);

-- Part of a Mercado Pago payment can be refunded; refunded_at still marks the full refund.
ALTER TABLE mp_charges ADD COLUMN refunded_cents integer NOT NULL DEFAULT 0;

ALTER TABLE stock_movements DROP CONSTRAINT stock_movements_reason_check;
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_reason_check
    CHECK (reason IN ('initial', 'purchase', 'sale', 'void', 'adjustment', 'loss', 'consumption', 'return'));
