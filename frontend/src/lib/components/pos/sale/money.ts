/** "1,250.50" → 125050. Returns null when empty or not a valid non-negative amount. */
export function toCents(text: string): number | null {
  const clean = text.replace(/[$\s,]/g, '');
  if (clean === '') return null;
  const n = Number(clean);
  if (!Number.isFinite(n) || n < 0) return null;
  return Math.round(n * 100);
}

/** 125050 → "1250.5" (editable text, no thousands separators). */
export const centsText = (cents: number | null | undefined) => (cents == null ? '' : String(cents / 100));

/** Quantities: integers plain, decimals up to 3 places. */
export const qtyText = (n: number) => (Number.isInteger(n) ? String(n) : n.toLocaleString('es-MX', { maximumFractionDigits: 3 }));

export const esc = (s: string) => s.replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c]!);
