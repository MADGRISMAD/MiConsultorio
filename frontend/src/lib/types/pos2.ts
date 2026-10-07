// Cobros v2: lots and expiry, consumables, commissions, abonos and CFDI stamping.

export interface StockLot {
  id: string;
  lot_code: string;
  expires_on: string | null;
  qty: number;
  expired: boolean;
}

export interface LotAlert {
  item_id: string;
  name: string;
  unit: string;
  lot_id: string;
  lot_code: string;
  expires_on: string;
  qty: number;
  /** negative when already expired */
  days_left: number;
}

export interface LowAlert {
  item_id: string;
  name: string;
  unit: string;
  stock: number;
  min_stock: number;
}

export interface PosAlerts {
  days: number;
  expiring: LotAlert[];
  expired: LotAlert[];
  low: LowAlert[];
}

export interface Consumable {
  product_id: string;
  name: string;
  unit: string;
  qty: number;
}

export interface Professional {
  id: string;
  name: string;
  role: string;
}

export interface CommissionRule {
  id: string;
  user_id: string | null;
  user_name: string;
  item_id: string | null;
  item_name: string;
  category: string | null;
  percent: number;
  active: boolean;
}

export interface CommissionRuleInput {
  user_id?: string;
  item_id?: string;
  category?: string;
  percent: number;
  active?: boolean;
}

export interface CommissionLine {
  folio: number;
  date: string;
  professional_id: string | null;
  professional: string;
  item: string;
  base_cents: number;
  percent: number;
  commission_cents: number;
}

export interface CommissionTotal {
  professional_id: string | null;
  professional: string;
  lines: number;
  base_cents: number;
  commission_cents: number;
}

export interface CommissionReport {
  from: string;
  to: string;
  totals: CommissionTotal[];
  lines: CommissionLine[];
  commission_cents: number;
}

export interface ReceivableSale {
  id: string;
  folio: number;
  created_at: string;
  total_cents: number;
  balance_cents: number;
  age_days: number;
}

export interface ReceivableCustomer {
  key: string;
  patient_id: string | null;
  name: string;
  balance_cents: number;
  oldest_days: number;
  sales: ReceivableSale[];
}

export interface Receivables {
  customers: ReceivableCustomer[];
  total_cents: number;
  aging: { '0_30': number; '31_60': number; '61_90': number; over_90: number };
}

export interface VoidResult {
  ok: boolean;
  refund_cents: number;
  refund_by_method: { method: string; amount_cents: number; count: number }[];
}

export interface StampResult {
  ok: boolean;
  fiscal_uuid: string;
  emailed: boolean;
  warnings: string[];
}

/** What the cobro page needs from an appointment (the agenda may send more). */
export interface ApptPrefill {
  id: string;
  patient_id: string | null;
  names: string;
  last_names: string;
  professional_id?: string | null;
  service_id?: string | null;
  sale_id?: string | null;
}

export interface PlanPrefillItem {
  id: string;
  phase: number;
  description: string;
  tooth: string;
  catalog_item_id: string | null;
  qty: number;
  unit_price_cents: number;
  tax_rate: number;
  status: 'pending' | 'done' | 'cancelled';
  sale_id?: string | null;
}

export interface PlanPrefill {
  id: string;
  title: string;
  patient_id: string;
  patient_name: string;
  status: string;
  total_cents: number;
  items: PlanPrefillItem[];
}

// ---- Devoluciones ----
export interface ReturnableLine {
  sale_item_id: string;
  name: string;
  kind: string;
  qty: number;
  returned_qty: number;
  returnable_qty: number;
  returnable_cents: number;
  unit_cents: number;
  tracks_stock: boolean;
}

export interface PastReturn {
  id: string;
  folio: number;
  total_cents: number;
  reason: string;
  credit_note_pending: boolean;
  created_by_name: string;
  created_at: string;
}

export interface ReturnInfo {
  sale: { id: string; folio: number; status: string; total_cents: number; invoiced: boolean };
  lines: ReturnableLine[];
  refundable: { method: string; amount_cents: number; count: number }[];
  returns: PastReturn[];
  can_return: boolean;
  returned_cents: number;
}

export interface ReturnInput {
  reason: string;
  lines: { sale_item_id: string; qty: number; restock?: boolean }[];
  refunds?: { method: string; amount_cents: number }[];
}

export interface ReturnResult {
  id: string;
  folio: number;
  sale_id: string;
  sale_folio: number;
  total_cents: number;
  reason: string;
  created_at: string;
  created_by_name: string;
  credit_note_pending: boolean;
  lines: { name: string; qty: number; amount_cents: number; restocked: boolean }[];
  refunds: { method: string; amount_cents: number }[];
}
