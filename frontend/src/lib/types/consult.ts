// Pre-account of a consultation (services and supplies sent to the register) and CFDI payment complements.

export type ChargeStatus = 'draft' | 'sent' | 'charged' | 'cancelled';

export interface ChargeItem {
  id: string;
  catalog_item_id: string | null;
  kind: 'service' | 'product';
  name: string;
  qty: number;
  unit_price_cents: number;
  tax_rate: number;
  discount_cents: number;
  total_cents: number;
  note: string;
  consumed: boolean;
  consumed_at: string | null;
}

export interface Charge {
  id: string;
  encounter_id: string | null;
  patient_id: string;
  patient_name: string;
  appointment_id: string | null;
  professional_id: string | null;
  professional_name: string;
  status: ChargeStatus;
  note: string;
  sale_id: string | null;
  created_by: string;
  cancel_reason?: string;
  sent_at: string | null;
  charged_at: string | null;
  created_at: string;
  updated_at: string;
  total_cents: number;
  item_count: number;
  items?: ChargeItem[];
  warnings?: string[];
}

/** A line while it is being edited (before it is saved). */
export interface ChargeDraftLine {
  key: string;
  /** id of the saved line (consumed lines are locked) */
  id?: string;
  catalog_item_id?: string;
  kind: 'service' | 'product';
  name: string;
  qty: number;
  unit: string;
  price_cents: number;
  note: string;
  consumed: boolean;
  no_charge: boolean;
  /** consumed lines already took their stock: they cannot be removed or changed */
  locked: boolean;
  track_stock: boolean;
  usable_stock?: number;
  warning?: string;
}

export interface ChargeCatalogItem {
  id: string;
  kind: 'service' | 'product';
  name: string;
  unit: string;
  price_cents: number;
  tax_rate: number;
  track_stock: boolean;
  stock: number;
  min_stock: number;
  next_expiry: string | null;
  expired_qty: number;
  usable_stock: number;
  stock_warning?: string;
}

export interface ChargeItemInput {
  id?: string;
  catalog_item_id?: string;
  name?: string;
  kind?: 'service' | 'product';
  qty: number;
  unit_price_cents?: number;
  note?: string;
  consumed?: boolean;
  no_charge?: boolean;
}

export interface ChargeInput {
  patient_id?: string;
  encounter_id?: string;
  appointment_id?: string;
  professional_id?: string;
  note?: string;
  items: ChargeItemInput[];
  send?: boolean;
}

export interface PaymentComplementRow {
  payment_id: string;
  method: string;
  amount_cents: number;
  paid_at: string;
  installment: number;
  previous_cents: number;
  balance_cents: number;
  can_issue: boolean;
  problem?: string;
  complement_id: string | null;
  complement_uuid: string;
  state: '' | 'stamping' | 'stamped';
  stamped_at: string | null;
}

export interface PaymentComplements {
  payments: PaymentComplementRow[];
  on_credit: boolean;
  stamped: boolean;
}

export interface ComplementResult {
  ok: boolean;
  fiscal_uuid: string;
  installment: number;
  emailed: boolean;
  warnings: string[];
}
