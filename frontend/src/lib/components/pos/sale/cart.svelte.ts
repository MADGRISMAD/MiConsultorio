import type { CatalogItem, PosSettings, SaleInput, SalePaymentInput } from '$lib/types';

export interface CartLine {
  key: string;
  /** Catalog item id; absent for free-form lines. */
  item_id?: string;
  name: string;
  unit: string;
  price_cents: number;
  /** Catalog price when the line was added, to know whether the price was edited. */
  base_price_cents: number;
  /** Percent (0-100), prices include tax. */
  tax_rate: number;
  qty: number;
  discount_cents: number;
  track_stock: boolean;
  stock: number;
}

export interface Person {
  name: string;
  curp: string;
}

const DRAFT_KEY = 'caresia.pos.draft.v1';
const uid = () => Math.random().toString(36).slice(2, 10);

/** Gross amount of a line, rounded like the server does (qty has up to 3 decimals). */
export const lineGross = (l: Pick<CartLine, 'price_cents' | 'qty'>) => Math.round((l.price_cents * Math.round(l.qty * 1000)) / 1000);

/** In-memory sale being built, with a localStorage draft so a refresh does not lose it. */
export class Cart {
  lines = $state<CartLine[]>([]);
  discMode = $state<'amount' | 'pct'>('amount');
  discAmount = $state<number | null>(null);
  discPct = $state<number | null>(null);
  customer = $state('');
  curp = $state('');
  note = $state('');

  // Set from the business settings
  defaultTax = $state(0);
  allowNegative = $state(false);
  allowDiscounts = $state(true);
  maxDiscountPct = $state(100);

  gross = $derived(this.lines.reduce((a, l) => a + lineGross(l), 0));
  lineDisc = $derived(this.lines.reduce((a, l) => a + (this.allowDiscounts ? l.discount_cents : 0), 0));
  base = $derived(this.gross - this.lineDisc);
  ticketDisc = $derived.by(() => {
    if (!this.allowDiscounts) return 0;
    const raw = this.discMode === 'pct' ? Math.round((this.base * Math.min(this.discPct ?? 0, 100)) / 100) : (this.discAmount ?? 0);
    return Math.max(0, Math.min(raw, this.base));
  });
  discountTotal = $derived(this.lineDisc + this.ticketDisc);
  total = $derived(this.base - this.ticketDisc);
  /** Estimated included tax (same split the server makes). */
  tax = $derived.by(() => {
    let t = 0;
    for (const l of this.lines) {
      let share = lineGross(l) - (this.allowDiscounts ? l.discount_cents : 0);
      if (this.base > 0 && this.ticketDisc > 0) share = Math.round((share * (this.base - this.ticketDisc)) / this.base);
      const r = Math.round(l.tax_rate * 100);
      t += Math.round((share * r) / (10000 + r));
    }
    return t;
  });
  count = $derived(this.lines.reduce((a, l) => a + l.qty, 0));
  empty = $derived(this.lines.length === 0);

  /** Why the sale cannot be charged yet (null when fine). */
  problem = $derived.by((): string | null => {
    if (this.empty) return 'Agrega al menos un concepto.';
    if (this.lines.some((l) => !(l.qty > 0))) return 'Revisa las cantidades.';
    if (this.lines.some((l) => lineGross(l) < l.discount_cents)) return 'Un descuento supera el importe de su concepto.';
    if (this.allowDiscounts && this.discMode === 'amount' && (this.discAmount ?? 0) > this.base) return 'El descuento supera el total.';
    if (this.total <= 0) return 'El total debe ser mayor a cero.';
    if (this.allowDiscounts && this.gross > 0 && this.maxDiscountPct < 100 && this.discountTotal * 100 > this.gross * this.maxDiscountPct)
      return `El descuento supera el máximo permitido (${this.maxDiscountPct} %).`;
    return null;
  });

  configure(s: PosSettings) {
    this.defaultTax = s.default_tax_rate;
    this.allowNegative = s.allow_negative_stock;
    this.allowDiscounts = s.allow_discounts;
    this.maxDiscountPct = s.max_discount_pct;
  }

  /** Largest quantity allowed for a line (Infinity when stock is not tracked). */
  maxQty(l: CartLine): number {
    return l.track_stock && !this.allowNegative ? Math.max(0, l.stock) : Infinity;
  }

  /** Adds one unit of a catalog item. Returns a warning when stock does not allow it. */
  add(item: CatalogItem, qty = 1): string | null {
    const existing = this.lines.find((l) => l.item_id === item.id && l.price_cents === item.price_cents && l.discount_cents === 0);
    const next = (existing?.qty ?? 0) + qty;
    const max = item.track_stock && !this.allowNegative ? Math.max(0, item.stock) : Infinity;
    if (next > max) return max <= 0 ? `«${item.name}» está agotado.` : `Solo quedan ${max} de «${item.name}».`;
    if (existing) existing.qty = Math.round(next * 1000) / 1000;
    else
      this.lines.push({
        key: uid(),
        item_id: item.id,
        name: item.name,
        unit: item.unit || 'pza',
        price_cents: item.price_cents,
        base_price_cents: item.price_cents,
        tax_rate: item.tax_rate,
        qty,
        discount_cents: 0,
        track_stock: item.track_stock,
        stock: item.stock
      });
    return null;
  }

  addFree(name: string, price_cents: number, tax_rate: number) {
    this.lines.push({ key: uid(), name, unit: 'pza', price_cents, base_price_cents: price_cents, tax_rate, qty: 1, discount_cents: 0, track_stock: false, stock: 0 });
  }

  remove(key: string) {
    this.lines = this.lines.filter((l) => l.key !== key);
  }

  clear() {
    this.lines = [];
    this.discAmount = null;
    this.discPct = null;
    this.discMode = 'amount';
    this.customer = '';
    this.curp = '';
    this.note = '';
    this.#store(null);
  }

  /** Refreshes stock figures after the catalog is reloaded. */
  syncStock(items: CatalogItem[]) {
    const byId = new Map(items.map((i) => [i.id, i]));
    for (const l of this.lines) {
      const it = l.item_id ? byId.get(l.item_id) : undefined;
      if (it) {
        l.stock = it.stock;
        l.track_stock = it.track_stock;
      }
    }
  }

  toInput(payments: SalePaymentInput[]): SaleInput {
    return {
      lines: this.lines.map((l) => ({
        ...(l.item_id ? { item_id: l.item_id } : { name: l.name, tax_rate: l.tax_rate }),
        qty: l.qty,
        // Only send the price when the person changed it (or for free-form lines)
        ...(!l.item_id || l.price_cents !== l.base_price_cents ? { unit_price_cents: l.price_cents } : {}),
        discount_cents: this.allowDiscounts ? l.discount_cents : 0
      })),
      discount_cents: this.ticketDisc,
      customer_name: this.customer.trim() || undefined,
      customer_curp: this.curp || undefined,
      note: this.note.trim() || undefined,
      payments
    };
  }

  // ---- draft ----
  snapshot() {
    return {
      lines: $state.snapshot(this.lines),
      discMode: this.discMode,
      discAmount: this.discAmount,
      discPct: this.discPct,
      customer: this.customer,
      curp: this.curp,
      note: this.note
    };
  }

  save() {
    const s = this.snapshot();
    this.#store(s.lines.length || s.customer || s.note ? s : null);
  }

  restore() {
    try {
      const raw = localStorage.getItem(DRAFT_KEY);
      if (!raw) return;
      const d = JSON.parse(raw);
      if (!Array.isArray(d.lines)) return;
      this.lines = d.lines.filter((l: CartLine) => l && typeof l.name === 'string' && l.price_cents >= 0 && l.qty > 0);
      this.discMode = d.discMode === 'pct' ? 'pct' : 'amount';
      this.discAmount = typeof d.discAmount === 'number' ? d.discAmount : null;
      this.discPct = typeof d.discPct === 'number' ? d.discPct : null;
      this.customer = String(d.customer ?? '');
      this.curp = String(d.curp ?? '');
      this.note = String(d.note ?? '');
    } catch {
      /* corrupt or unavailable draft: start empty */
    }
  }

  #store(v: unknown) {
    try {
      if (v) localStorage.setItem(DRAFT_KEY, JSON.stringify(v));
      else localStorage.removeItem(DRAFT_KEY);
    } catch {
      /* storage unavailable */
    }
  }
}
