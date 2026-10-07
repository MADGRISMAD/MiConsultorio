import type { CatalogInput, CatalogItem, ItemKind } from '$lib/types';

export const UNITS = ['pza', 'caja', 'sesión', 'consulta', 'ml', 'g', 'kg', 'l', 'paquete', 'frasco', 'tableta', 'dosis'];

export const REASON_LABELS: Record<string, string> = {
  initial: 'Existencia inicial',
  purchase: 'Compra / entrada',
  sale: 'Venta',
  void: 'Venta cancelada',
  adjustment: 'Ajuste / conteo',
  loss: 'Salida / merma'
};

/** Parses a peso amount typed by a person ("1,250.50", "1.250,50", "$80", "80,5"). NaN when invalid. */
export function parsePesos(raw: string | number): number {
  if (typeof raw === 'number') return raw;
  let s = raw.replace(/[$\s]/g, '').replace(/mxn/i, '');
  if (!s) return NaN;
  const lastDot = s.lastIndexOf('.');
  const lastComma = s.lastIndexOf(',');
  if (lastDot >= 0 && lastComma >= 0) {
    // both present: the last one is the decimal separator
    if (lastComma > lastDot) s = s.replace(/\./g, '').replace(',', '.');
    else s = s.replace(/,/g, '');
  } else if (lastComma >= 0) {
    s = /,\d{1,2}$/.test(s) && s.indexOf(',') === lastComma ? s.replace(',', '.') : s.replace(/,/g, '');
  }
  if (!/^-?\d*\.?\d+$/.test(s)) return NaN;
  return Number(s);
}

export const toCents = (pesos: number) => Math.round(pesos * 100);
export const toPesos = (cents: number) => (cents ? String(cents / 100) : '');

/** Margin over price, as a percentage; null when it cannot be computed. */
export function marginPct(priceCents: number, costCents: number): number | null {
  if (!costCents || !priceCents || priceCents <= 0) return null;
  return ((priceCents - costCents) / priceCents) * 100;
}

export const pct = (n: number) => `${n.toLocaleString('es-MX', { maximumFractionDigits: 1 })} %`;
export const qty = (n: number) => n.toLocaleString('es-MX', { maximumFractionDigits: 3 });

export const normalize = (s: string) => s.toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '').trim();

export function blankInput(kind: ItemKind = 'service'): CatalogInput {
  return { kind, name: '', sku: '', barcode: '', category: '', price_cents: 0, cost_cents: 0, tax_rate: 0, track_stock: false, min_stock: 0, unit: kind === 'service' ? 'sesión' : 'pza' };
}

export function toInput(i: CatalogItem, patch: Partial<CatalogInput> = {}): CatalogInput {
  const { id: _id, ...rest } = i;
  return { ...rest, ...patch };
}

export const isLow = (i: CatalogItem) => i.track_stock && i.stock <= i.min_stock;

// ---------- CSV / spreadsheet paste ----------

/** Splits pasted text into rows of cells. Detects tab, semicolon or comma; supports "quoted, cells". */
export function parseDelimited(text: string): string[][] {
  const clean = text.replace(/^﻿/, '').replace(/\r\n?/g, '\n');
  const firstLine = clean.split('\n').find((l) => l.trim()) ?? '';
  const delim = firstLine.includes('\t') ? '\t' : firstLine.includes(';') ? ';' : ',';
  const rows: string[][] = [];
  let row: string[] = [];
  let cell = '';
  let quoted = false;
  for (let i = 0; i < clean.length; i++) {
    const c = clean[i];
    if (quoted) {
      if (c === '"' && clean[i + 1] === '"') {
        cell += '"';
        i++;
      } else if (c === '"') quoted = false;
      else cell += c;
    } else if (c === '"') quoted = true;
    else if (c === delim) {
      row.push(cell.trim());
      cell = '';
    } else if (c === '\n') {
      row.push(cell.trim());
      if (row.some((x) => x)) rows.push(row);
      row = [];
      cell = '';
    } else cell += c;
  }
  row.push(cell.trim());
  if (row.some((x) => x)) rows.push(row);
  return rows;
}

const FIELDS = ['tipo', 'nombre', 'categoria', 'precio', 'costo', 'existencia', 'unidad', 'barras'] as const;
type Field = (typeof FIELDS)[number];

function headerField(cell: string): Field | null {
  const c = normalize(cell);
  if (/^(tipo|kind)$/.test(c)) return 'tipo';
  if (/^(nombre|name|articulo|producto|servicio|concepto|descripcion)$/.test(c)) return 'nombre';
  if (/^(categoria|category|rubro)$/.test(c)) return 'categoria';
  if (/^(precio|price|precio publico|precio venta)$/.test(c)) return 'precio';
  if (/^(costo|cost)$/.test(c)) return 'costo';
  if (/^(existencia|existencias|stock|cantidad|inventario)$/.test(c)) return 'existencia';
  if (/^(unidad|unit)$/.test(c)) return 'unidad';
  if (/^(codigo de barras|codigo barras|barras|barcode|ean|upc|codigo)$/.test(c)) return 'barras';
  return null;
}

export interface ImportRow {
  line: number;
  input: CatalogInput;
  errors: string[];
}

/** Turns pasted text into catalog rows with per-row validation messages (Spanish). */
export function parseCatalogText(text: string): ImportRow[] {
  const rows = parseDelimited(text);
  if (!rows.length) return [];
  let order: (Field | null)[] = [...FIELDS];
  let start = 0;
  const mapped = rows[0].map(headerField);
  if (mapped.filter(Boolean).length >= 2) {
    order = mapped;
    start = 1;
  }
  const out: ImportRow[] = [];
  for (let r = start; r < rows.length; r++) {
    const cells = rows[r];
    const get = (f: Field) => {
      const idx = order.indexOf(f);
      return idx >= 0 ? (cells[idx] ?? '').trim() : '';
    };
    const errors: string[] = [];
    const name = get('nombre');
    if (!name) errors.push('Falta el nombre');
    const priceRaw = get('precio');
    const price = priceRaw === '' ? 0 : parsePesos(priceRaw);
    if (Number.isNaN(price) || price < 0) errors.push('Precio no válido');
    const costRaw = get('costo');
    const cost = costRaw === '' ? 0 : parsePesos(costRaw);
    if (Number.isNaN(cost) || cost < 0) errors.push('Costo no válido');
    const stockRaw = get('existencia');
    const stock = stockRaw === '' ? 0 : parsePesos(stockRaw);
    if (Number.isNaN(stock) || stock < 0) errors.push('Existencia no válida');
    const t = normalize(get('tipo'));
    const kind: ItemKind = t ? (t.startsWith('p') || t === 'product' ? 'product' : 'service') : stockRaw !== '' ? 'product' : 'service';
    const track = kind === 'product' && stockRaw !== '' && stock > 0;
    out.push({
      line: r + 1,
      errors,
      input: {
        ...blankInput(kind),
        name,
        category: get('categoria'),
        barcode: get('barras'),
        unit: get('unidad') || (kind === 'service' ? 'sesión' : 'pza'),
        price_cents: Number.isNaN(price) ? 0 : toCents(price),
        cost_cents: Number.isNaN(cost) ? 0 : toCents(cost),
        track_stock: track,
        stock: track ? stock : undefined
      }
    });
  }
  return out;
}

// ---------- photo for Inventario Mágico ----------

export const MAGIC_MIMES = ['image/jpeg', 'image/png', 'image/webp'];
export const MAGIC_MAX_BYTES = 5 * 1024 * 1024;

/** Reads an image file and downsizes it to at most `max` px on the long edge. Returns base64 (no prefix). */
export async function imageToBase64(file: File, max = 1600): Promise<{ base64: string; mime: string; preview: string }> {
  const dataUrl = await new Promise<string>((resolve, reject) => {
    const fr = new FileReader();
    fr.onload = () => resolve(String(fr.result));
    fr.onerror = () => reject(new Error('No se pudo leer la imagen.'));
    fr.readAsDataURL(file);
  });
  const img = await new Promise<HTMLImageElement>((resolve, reject) => {
    const im = new Image();
    im.onload = () => resolve(im);
    im.onerror = () => reject(new Error('La imagen no se pudo abrir.'));
    im.src = dataUrl;
  });
  const scale = Math.min(1, max / Math.max(img.width, img.height));
  const canvas = document.createElement('canvas');
  canvas.width = Math.max(1, Math.round(img.width * scale));
  canvas.height = Math.max(1, Math.round(img.height * scale));
  const ctx = canvas.getContext('2d');
  if (!ctx) throw new Error('Tu navegador no pudo procesar la imagen.');
  ctx.fillStyle = '#fff';
  ctx.fillRect(0, 0, canvas.width, canvas.height);
  ctx.drawImage(img, 0, 0, canvas.width, canvas.height);
  const out = canvas.toDataURL('image/jpeg', 0.85);
  return { base64: out.split(',')[1] ?? '', mime: 'image/jpeg', preview: out };
}
