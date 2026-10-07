-- Pre-account (cuenta abierta) inside the consultation: the professional lists the services and supplies
-- used and sends them to the register, which charges them. Also the CFDI payment complement (Pago 2.0)
-- for the abonos of sales invoiced as PPD.

CREATE TABLE encounter_charges (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id        uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    encounter_id     uuid REFERENCES encounters (id) ON DELETE SET NULL, -- NULL until the note is saved
    patient_id       uuid NOT NULL REFERENCES patients (id) ON DELETE CASCADE,
    appointment_id   uuid REFERENCES appointments (id) ON DELETE SET NULL,
    professional_id  uuid REFERENCES users (id) ON DELETE SET NULL,
    status           text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'sent', 'charged', 'cancelled')),
    note             text NOT NULL DEFAULT '',
    sale_id          uuid REFERENCES sales (id) ON DELETE SET NULL,
    created_by       uuid REFERENCES users (id) ON DELETE SET NULL,
    created_by_name  text NOT NULL DEFAULT '',
    cancel_reason    text NOT NULL DEFAULT '',
    sent_at          timestamptz,
    charged_at       timestamptz,
    cancelled_at     timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX encounter_charges_clinic_idx ON encounter_charges (clinic_id, status, updated_at DESC);
CREATE INDEX encounter_charges_encounter_idx ON encounter_charges (encounter_id) WHERE encounter_id IS NOT NULL;
CREATE INDEX encounter_charges_patient_idx ON encounter_charges (patient_id);

CREATE TABLE encounter_charge_items (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    charge_id        uuid NOT NULL REFERENCES encounter_charges (id) ON DELETE CASCADE,
    clinic_id        uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    catalog_item_id  uuid REFERENCES catalog_items (id) ON DELETE SET NULL,
    kind             text NOT NULL CHECK (kind IN ('service', 'product')),
    name             text NOT NULL,            -- snapshot
    qty              numeric(12, 3) NOT NULL CHECK (qty > 0),
    unit_price_cents integer NOT NULL DEFAULT 0 CHECK (unit_price_cents >= 0), -- snapshot; 0 in plans without cobros or for supplies not charged
    tax_rate         numeric(5, 2) NOT NULL DEFAULT 0,
    discount_cents   integer NOT NULL DEFAULT 0 CHECK (discount_cents >= 0),
    note             text NOT NULL DEFAULT '',
    consumed         boolean NOT NULL DEFAULT false, -- stock already taken out when it was marked (never twice)
    consumed_at      timestamptz,
    position         integer NOT NULL DEFAULT 0
);
CREATE INDEX encounter_charge_items_charge_idx ON encounter_charge_items (charge_id, position);

-- A consumption made for a pre-account line, and the sale that billed a consumption (so it is not taken again).
ALTER TABLE stock_movements
    ADD COLUMN charge_item_id uuid REFERENCES encounter_charge_items (id) ON DELETE SET NULL,
    ADD COLUMN billed_sale_id uuid REFERENCES sales (id) ON DELETE SET NULL;
CREATE INDEX stock_movements_encounter_idx ON stock_movements (encounter_id) WHERE encounter_id IS NOT NULL;

-- One complement per abono. The unique index is what prevents duplicates; a failed stamp removes its row.
CREATE TABLE invoice_payment_complements (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id       uuid NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    invoice_id      uuid NOT NULL REFERENCES invoice_requests (id) ON DELETE CASCADE,
    sale_id         uuid NOT NULL REFERENCES sales (id) ON DELETE CASCADE,
    payment_id      uuid NOT NULL REFERENCES sale_payments (id) ON DELETE CASCADE,
    state           text NOT NULL DEFAULT 'stamping' CHECK (state IN ('stamping', 'stamped')),
    installment     integer NOT NULL,           -- NumParcialidad
    previous_cents  integer NOT NULL,           -- ImpSaldoAnt
    paid_cents      integer NOT NULL,           -- ImpPagado
    balance_cents   integer NOT NULL,           -- ImpSaldoInsoluto
    related_uuid    text NOT NULL,              -- UUID of the PPD invoice
    fiscal_uuid     text NOT NULL DEFAULT '',   -- UUID of the complement
    cfdi_id         text NOT NULL DEFAULT '',
    xml_path        text NOT NULL DEFAULT '',
    pdf_path        text NOT NULL DEFAULT '',
    email           text NOT NULL DEFAULT '',
    emailed         boolean NOT NULL DEFAULT false,
    created_by_name text NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now(),
    stamped_at      timestamptz
);
CREATE UNIQUE INDEX invoice_payment_complements_payment ON invoice_payment_complements (payment_id);
CREATE INDEX invoice_payment_complements_invoice_idx ON invoice_payment_complements (invoice_id);
