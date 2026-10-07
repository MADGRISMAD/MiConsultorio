-- Cobros (point of sale): catalog, inventory, sales, cash register, invoice requests.
-- Money is stored in cents (integer). Prices already include tax.

ALTER TABLE clinics
    ADD COLUMN sale_seq     integer NOT NULL DEFAULT 0,
    ADD COLUMN pos_settings jsonb   NOT NULL DEFAULT '{}';

CREATE TABLE catalog_items (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id   uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    kind        text NOT NULL CHECK (kind IN ('service', 'product')),
    name        text NOT NULL,
    sku         text NOT NULL DEFAULT '',
    barcode     text NOT NULL DEFAULT '',
    category    text NOT NULL DEFAULT '',
    price_cents integer NOT NULL CHECK (price_cents >= 0),
    cost_cents  integer NOT NULL DEFAULT 0 CHECK (cost_cents >= 0),
    tax_rate    numeric(5, 2) NOT NULL DEFAULT 0 CHECK (tax_rate >= 0 AND tax_rate <= 100),
    track_stock boolean NOT NULL DEFAULT false,
    stock       numeric(12, 3) NOT NULL DEFAULT 0,
    min_stock   numeric(12, 3) NOT NULL DEFAULT 0,
    unit        text NOT NULL DEFAULT 'pza',
    active      boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX catalog_items_clinic_idx ON catalog_items (clinic_id, kind, active);
CREATE UNIQUE INDEX catalog_items_sku_key ON catalog_items (clinic_id, lower(sku)) WHERE sku <> '';
CREATE UNIQUE INDEX catalog_items_barcode_key ON catalog_items (clinic_id, barcode) WHERE barcode <> '';

CREATE TABLE cash_sessions (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id      uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    opened_by      uuid REFERENCES users (id) ON DELETE SET NULL,
    opened_by_name text NOT NULL DEFAULT '',
    opened_at      timestamptz NOT NULL DEFAULT now(),
    opening_cents  integer NOT NULL DEFAULT 0 CHECK (opening_cents >= 0),
    closed_by      uuid REFERENCES users (id) ON DELETE SET NULL,
    closed_by_name text NOT NULL DEFAULT '',
    closed_at      timestamptz,
    counted_cents  integer,
    expected_cents integer,
    note           text NOT NULL DEFAULT ''
);
CREATE INDEX cash_sessions_clinic_idx ON cash_sessions (clinic_id, opened_at DESC);
-- only one open register per clinic
CREATE UNIQUE INDEX cash_sessions_one_open ON cash_sessions (clinic_id) WHERE closed_at IS NULL;

CREATE TABLE cash_movements (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id   uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    session_id  uuid NOT NULL REFERENCES cash_sessions (id) ON DELETE CASCADE,
    kind        text NOT NULL CHECK (kind IN ('in', 'out')),
    amount_cents integer NOT NULL CHECK (amount_cents > 0),
    concept     text NOT NULL,
    created_by  uuid REFERENCES users (id) ON DELETE SET NULL,
    created_by_name text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX cash_movements_session_idx ON cash_movements (session_id);

CREATE TABLE sales (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id      uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    folio          integer NOT NULL,
    session_id     uuid REFERENCES cash_sessions (id) ON DELETE SET NULL,
    customer_name  text NOT NULL DEFAULT '',
    customer_curp  text NOT NULL DEFAULT '',
    appointment_id uuid,
    note           text NOT NULL DEFAULT '',
    subtotal_cents integer NOT NULL,
    discount_cents integer NOT NULL DEFAULT 0,
    tax_cents      integer NOT NULL DEFAULT 0,
    total_cents    integer NOT NULL,
    status         text NOT NULL DEFAULT 'paid' CHECK (status IN ('paid', 'void')),
    void_reason    text NOT NULL DEFAULT '',
    voided_at      timestamptz,
    voided_by_name text NOT NULL DEFAULT '',
    created_by     uuid REFERENCES users (id) ON DELETE SET NULL,
    created_by_name text NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (clinic_id, folio)
);
CREATE INDEX sales_clinic_date_idx ON sales (clinic_id, created_at DESC);
CREATE INDEX sales_session_idx ON sales (session_id);

CREATE TABLE sale_items (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    sale_id          uuid NOT NULL REFERENCES sales (id) ON DELETE CASCADE,
    item_id          uuid REFERENCES catalog_items (id) ON DELETE SET NULL,
    kind             text NOT NULL DEFAULT 'service',
    name             text NOT NULL,
    qty              numeric(12, 3) NOT NULL CHECK (qty > 0),
    unit_price_cents integer NOT NULL CHECK (unit_price_cents >= 0),
    unit_cost_cents  integer NOT NULL DEFAULT 0,
    tax_rate         numeric(5, 2) NOT NULL DEFAULT 0,
    discount_cents   integer NOT NULL DEFAULT 0,
    total_cents      integer NOT NULL
);
CREATE INDEX sale_items_sale_idx ON sale_items (sale_id);
CREATE INDEX sale_items_item_idx ON sale_items (item_id);

CREATE TABLE sale_payments (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    sale_id        uuid NOT NULL REFERENCES sales (id) ON DELETE CASCADE,
    method         text NOT NULL CHECK (method IN ('cash', 'card', 'transfer', 'mp_point', 'mp_link', 'other')),
    amount_cents   integer NOT NULL CHECK (amount_cents > 0),
    received_cents integer,
    change_cents   integer NOT NULL DEFAULT 0,
    reference      text NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sale_payments_sale_idx ON sale_payments (sale_id);

CREATE TABLE stock_movements (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id  uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    item_id    uuid NOT NULL REFERENCES catalog_items (id) ON DELETE CASCADE,
    delta      numeric(12, 3) NOT NULL,
    reason     text NOT NULL CHECK (reason IN ('initial', 'purchase', 'sale', 'void', 'adjustment', 'loss')),
    sale_id    uuid REFERENCES sales (id) ON DELETE SET NULL,
    note       text NOT NULL DEFAULT '',
    balance    numeric(12, 3) NOT NULL,
    created_by_name text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX stock_movements_item_idx ON stock_movements (item_id, created_at DESC);

-- Fiscal invoice requests: captured per sale; stamping happens outside (PAC) and is marked here.
CREATE TABLE invoice_requests (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id     uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    sale_id       uuid NOT NULL REFERENCES sales (id) ON DELETE CASCADE,
    rfc           text NOT NULL,
    legal_name    text NOT NULL,
    tax_regime    text NOT NULL DEFAULT '',
    zip_code      text NOT NULL DEFAULT '',
    cfdi_use      text NOT NULL DEFAULT 'G03',
    email         text NOT NULL DEFAULT '',
    status        text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'issued', 'cancelled')),
    fiscal_uuid   text NOT NULL DEFAULT '',
    note          text NOT NULL DEFAULT '',
    created_by_name text NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX invoice_requests_clinic_idx ON invoice_requests (clinic_id, created_at DESC);
CREATE UNIQUE INDEX invoice_requests_open_sale ON invoice_requests (sale_id) WHERE status <> 'cancelled';
