-- Cobros v2: stock lots with expiry dates, consumables per service, professionals and commissions,
-- partial payments (cuentas por cobrar) and CFDI stamping.

-- ---------------------------------------------------------------------------
-- Lots. catalog_items.stock stays as the total and always equals the sum of the item's lots.
-- ---------------------------------------------------------------------------
CREATE TABLE stock_lots (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id  uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    item_id    uuid NOT NULL REFERENCES catalog_items (id) ON DELETE CASCADE,
    lot_code   text NOT NULL,
    expires_on date,
    qty        numeric(12, 3) NOT NULL DEFAULT 0, -- may go negative only when the clinic allows negative stock
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX stock_lots_key ON stock_lots (item_id, lower(lot_code), coalesce(expires_on, DATE '0001-01-01'));
CREATE INDEX stock_lots_item_idx ON stock_lots (item_id, expires_on NULLS LAST);
CREATE INDEX stock_lots_expiry_idx ON stock_lots (clinic_id, expires_on) WHERE qty > 0 AND expires_on IS NOT NULL;

-- Existing stock goes into a "SIN LOTE" lot.
INSERT INTO stock_lots (clinic_id, item_id, lot_code, qty)
SELECT clinic_id, id, 'SIN LOTE', stock FROM catalog_items WHERE track_stock;

ALTER TABLE catalog_items
    ADD COLUMN sat_product_code text NOT NULL DEFAULT '', -- empty = default for its kind when stamping
    ADD COLUMN sat_unit_code    text NOT NULL DEFAULT '';

ALTER TABLE stock_movements
    ADD COLUMN lot_id       uuid REFERENCES stock_lots (id) ON DELETE SET NULL,
    ADD COLUMN lot_code     text NOT NULL DEFAULT '',
    ADD COLUMN encounter_id uuid REFERENCES encounters (id) ON DELETE SET NULL;
ALTER TABLE stock_movements DROP CONSTRAINT stock_movements_reason_check;
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_reason_check
    CHECK (reason IN ('initial', 'purchase', 'sale', 'void', 'adjustment', 'loss', 'consumption'));
CREATE INDEX stock_movements_sale_idx ON stock_movements (sale_id) WHERE sale_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- Supplies a service uses up every time it is sold.
-- ---------------------------------------------------------------------------
CREATE TABLE service_consumables (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id  uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    service_id uuid NOT NULL REFERENCES catalog_items (id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES catalog_items (id) ON DELETE CASCADE,
    qty        numeric(12, 3) NOT NULL CHECK (qty > 0),
    UNIQUE (service_id, product_id)
);
CREATE INDEX service_consumables_clinic_idx ON service_consumables (clinic_id);

-- ---------------------------------------------------------------------------
-- Professionals and commissions
-- ---------------------------------------------------------------------------
ALTER TABLE sales
    ADD COLUMN professional_id uuid REFERENCES users (id) ON DELETE SET NULL,
    ADD COLUMN patient_id      uuid REFERENCES patients (id) ON DELETE SET NULL,
    ADD COLUMN balance_cents   integer NOT NULL DEFAULT 0 CHECK (balance_cents >= 0),
    ADD COLUMN on_credit       boolean NOT NULL DEFAULT false; -- sold on account (abonos); invoiced as PPD
ALTER TABLE sales DROP CONSTRAINT sales_status_check;
ALTER TABLE sales ADD CONSTRAINT sales_status_check CHECK (status IN ('paid', 'open', 'void'));
CREATE INDEX sales_open_idx ON sales (clinic_id) WHERE status = 'open';
CREATE INDEX sales_professional_idx ON sales (clinic_id, professional_id);
CREATE INDEX sales_patient_idx ON sales (patient_id);

ALTER TABLE sale_items
    ADD COLUMN professional_id  uuid REFERENCES users (id) ON DELETE SET NULL,
    ADD COLUMN commission_pct   numeric(5, 2) NOT NULL DEFAULT 0,
    ADD COLUMN commission_base_cents integer NOT NULL DEFAULT 0, -- what the percentage was applied to (after discounts, without tax)
    ADD COLUMN commission_cents integer NOT NULL DEFAULT 0;

-- Money received goes into the register that is open when it arrives (abonos can come days later).
ALTER TABLE sale_payments ADD COLUMN session_id uuid REFERENCES cash_sessions (id) ON DELETE SET NULL;
UPDATE sale_payments sp SET session_id = s.session_id FROM sales s WHERE s.id = sp.sale_id;
CREATE INDEX sale_payments_session_idx ON sale_payments (session_id);

-- user_id NULL = everyone. item_id and category are mutually exclusive. Most specific rule wins:
-- item > category > professional > general (see commission_rules in pos_commissions.go).
CREATE TABLE commission_rules (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id  uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    user_id    uuid REFERENCES users (id) ON DELETE CASCADE,
    item_id    uuid REFERENCES catalog_items (id) ON DELETE CASCADE,
    category   text,
    percent    numeric(5, 2) NOT NULL CHECK (percent >= 0 AND percent <= 100),
    active     boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (item_id IS NULL OR category IS NULL)
);
CREATE INDEX commission_rules_clinic_idx ON commission_rules (clinic_id, active);

-- ---------------------------------------------------------------------------
-- CFDI stamping (Facturama). The invoice request keeps being the record; these columns hold the result.
-- cfdi_state: '' (not stamped through a PAC), 'stamping', 'stamped', 'cancelled'
-- ---------------------------------------------------------------------------
ALTER TABLE invoice_requests
    ADD COLUMN cfdi_state          text NOT NULL DEFAULT '' CHECK (cfdi_state IN ('', 'stamping', 'stamped', 'cancelled')),
    ADD COLUMN cfdi_id             text NOT NULL DEFAULT '', -- the PAC's identifier
    ADD COLUMN cfdi_xml_path       text NOT NULL DEFAULT '', -- relative to UPLOADS_DIR
    ADD COLUMN cfdi_pdf_path       text NOT NULL DEFAULT '',
    ADD COLUMN stamped_at          timestamptz,
    ADD COLUMN cancel_motive       text NOT NULL DEFAULT '',
    ADD COLUMN cancel_replacement  text NOT NULL DEFAULT '',
    ADD COLUMN cfdi_cancelled_at   timestamptz;
