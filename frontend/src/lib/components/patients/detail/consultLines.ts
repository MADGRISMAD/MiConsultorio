import type { Charge, ChargeCatalogItem, ChargeDraftLine, ChargeItemInput } from '$lib/types/consult';

const uid = () => Math.random().toString(36).slice(2, 10);

export const lineTotal = (l: Pick<ChargeDraftLine, 'price_cents' | 'qty' | 'no_charge'>) => (l.no_charge ? 0 : Math.round((l.price_cents * Math.round(l.qty * 1000)) / 1000));

export function draftFromCatalog(it: ChargeCatalogItem, consumed: boolean): ChargeDraftLine {
  return {
    key: uid(),
    catalog_item_id: it.id,
    kind: it.kind,
    name: it.name,
    qty: 1,
    unit: it.unit || 'pza',
    price_cents: it.price_cents,
    note: '',
    consumed,
    no_charge: false,
    locked: false,
    track_stock: it.track_stock,
    usable_stock: it.track_stock ? it.usable_stock : undefined,
    warning: it.stock_warning
  };
}

export function draftFree(name: string): ChargeDraftLine {
  return { key: uid(), kind: 'service', name, qty: 1, unit: '', price_cents: 0, note: '', consumed: false, no_charge: false, locked: false, track_stock: false };
}

/** Lines of a saved pre-account, ready to edit (consumed ones are locked). */
export function draftsFromCharge(c: Charge): ChargeDraftLine[] {
  return (c.items ?? []).map((i) => ({
    key: i.id,
    id: i.id,
    catalog_item_id: i.catalog_item_id ?? undefined,
    kind: i.kind,
    name: i.name,
    qty: i.qty,
    unit: '',
    price_cents: i.unit_price_cents,
    note: i.note,
    consumed: i.consumed,
    // a supply used but not billed has price 0 and was consumed
    no_charge: i.consumed && i.unit_price_cents === 0,
    locked: i.consumed,
    track_stock: i.consumed
  }));
}

export function toItems(lines: ChargeDraftLine[]): ChargeItemInput[] {
  return lines.map((l) => ({
    ...(l.id ? { id: l.id } : {}),
    ...(l.catalog_item_id ? { catalog_item_id: l.catalog_item_id } : { name: l.name, kind: l.kind }),
    qty: l.qty,
    note: l.note || undefined,
    ...(l.locked ? {} : { consumed: l.consumed || undefined, no_charge: l.no_charge || undefined })
  }));
}
