import type { Dentition, OdontogramData, Surface, ToothData, ToothState } from '$lib/types/specialty';

export interface StateDef {
  id: ToothState;
  label: string;
  key: string;
  /** applies to the whole tooth (otherwise to one surface) */
  whole: boolean;
  color: string;
  letter: string;
}

export const STATES: StateDef[] = [
  { id: 'caries', label: 'Caries', key: '1', whole: false, color: '#dc2626', letter: 'C' },
  { id: 'restauracion', label: 'Restauración', key: '2', whole: false, color: '#2563eb', letter: 'R' },
  { id: 'sellador', label: 'Sellador', key: '3', whole: false, color: '#16a34a', letter: 'S' },
  { id: 'fractura', label: 'Fractura', key: '4', whole: false, color: '#ea580c', letter: 'F' },
  { id: 'endodoncia', label: 'Endodoncia', key: '5', whole: true, color: '#9333ea', letter: 'E' },
  { id: 'corona', label: 'Corona', key: '6', whole: true, color: '#ca8a04', letter: 'K' },
  { id: 'extraccion_indicada', label: 'Extracción indicada', key: '7', whole: true, color: '#b91c1c', letter: 'X' },
  { id: 'ausente', label: 'Ausente / extraída', key: '8', whole: true, color: '#6b7280', letter: 'A' },
  { id: 'implante', label: 'Implante', key: '9', whole: true, color: '#0d9488', letter: 'I' },
  { id: 'protesis', label: 'Prótesis', key: '0', whole: true, color: '#92400e', letter: 'P' }
];
export const stateDef = (id: string | undefined) => STATES.find((s) => s.id === id);

export const SURFACE_NAMES: Record<Surface, string> = { V: 'Vestibular', L: 'Lingual', P: 'Palatina', M: 'Mesial', D: 'Distal', O: 'Oclusal', I: 'Incisal' };

export const CELL = 34;
export const GAP = 5;
const IN = 9;

export interface Row {
  arch: 'upper' | 'lower';
  deciduous: boolean;
  teeth: number[];
}

const range = (a: number, b: number) => {
  const out: number[] = [];
  for (let n = a; n !== b + Math.sign(b - a); n += Math.sign(b - a)) out.push(n);
  return out;
};

export function rowsFor(d: Dentition): Row[] {
  const adultU: Row = { arch: 'upper', deciduous: false, teeth: [...range(18, 11), ...range(21, 28)] };
  const adultL: Row = { arch: 'lower', deciduous: false, teeth: [...range(48, 41), ...range(31, 38)] };
  const childU: Row = { arch: 'upper', deciduous: true, teeth: [...range(55, 51), ...range(61, 65)] };
  const childL: Row = { arch: 'lower', deciduous: true, teeth: [...range(85, 81), ...range(71, 75)] };
  if (d === 'adult') return [adultU, adultL];
  if (d === 'child') return [childU, childL];
  return [adultU, childU, childL, adultL];
}

export const allTeeth = () => [...range(11, 18), ...range(21, 28), ...range(31, 38), ...range(41, 48), ...range(51, 55), ...range(61, 65), ...range(71, 75), ...range(81, 85)];

export const toothKind = (n: number) => {
  const u = n % 10;
  return u <= 2 ? 'Incisivo' : u === 3 ? 'Canino' : n >= 50 ? 'Molar temporal' : u <= 5 ? 'Premolar' : 'Molar';
};
export const isAnterior = (n: number) => n % 10 <= 3;
const quadrant = (n: number) => Math.floor(n / 10);
/** quadrants shown on the left of the chart (patient's right) */
const onLeft = (n: number) => [1, 4, 5, 8].includes(quadrant(n));
export const isUpper = (n: number) => [1, 2, 5, 6].includes(quadrant(n));

export interface SurfaceShape {
  surface: Surface;
  points: string;
}

/** Five surfaces of a tooth in a CELL x CELL box. Mesial faces the midline. */
export function surfaceShapes(n: number): SurfaceShape[] {
  const s = CELL;
  const upper = isUpper(n);
  const left = onLeft(n);
  const pts = (...p: number[]) => p.join(' ');
  const top = pts(0, 0, s, 0, s - IN, IN, IN, IN);
  const bottom = pts(0, s, s, s, s - IN, s - IN, IN, s - IN);
  const lft = pts(0, 0, IN, IN, IN, s - IN, 0, s);
  const rgt = pts(s, 0, s - IN, IN, s - IN, s - IN, s, s);
  const center = pts(IN, IN, s - IN, IN, s - IN, s - IN, IN, s - IN);
  return [
    { surface: upper ? 'V' : 'L', points: top },
    { surface: upper ? 'P' : 'V', points: bottom },
    { surface: left ? 'D' : 'M', points: lft },
    { surface: left ? 'M' : 'D', points: rgt },
    { surface: isAnterior(n) ? 'I' : 'O', points: center }
  ];
}

export function toothX(row: Row, index: number): number {
  const half = row.teeth.length / 2;
  return index * (CELL + GAP) + (index >= half ? 14 : 0);
}
export const rowWidth = (row: Row) => row.teeth.length * (CELL + GAP) + 14;
export const ROW_H = CELL + 30;

/** SVG fragment (no user input) drawn above a tooth for whole-tooth states. */
export function glyph(state: ToothState | undefined, s = CELL): string {
  const c = stateDef(state)?.color ?? '#000';
  const m = s / 2;
  switch (state) {
    case 'endodoncia':
      return `<path d="M${m} 6 V${s - 6}" stroke="${c}" stroke-width="3.5" stroke-linecap="round" fill="none"/>`;
    case 'corona':
      return `<rect x="2" y="2" width="${s - 4}" height="${s - 4}" rx="5" fill="none" stroke="${c}" stroke-width="3"/>`;
    case 'extraccion_indicada':
      return `<path d="M4 4 L${s - 4} ${s - 4} M${s - 4} 4 L4 ${s - 4}" stroke="${c}" stroke-width="3.5" stroke-linecap="round"/>`;
    case 'ausente':
      return `<path d="M4 ${m} H${s - 4}" stroke="${c}" stroke-width="3.5" stroke-linecap="round"/><path d="M${m} 4 V${s - 4}" stroke="${c}" stroke-width="3.5" stroke-linecap="round"/>`;
    case 'implante':
      return `<circle cx="${m}" cy="${m}" r="9" fill="none" stroke="${c}" stroke-width="3"/><circle cx="${m}" cy="${m}" r="2.5" fill="${c}"/>`;
    case 'protesis':
      return `<path d="M3 ${m} H${s - 3}" stroke="${c}" stroke-width="6" stroke-linecap="round" opacity=".85"/>`;
    case 'fractura':
      return `<path d="M8 4 L${m} ${m - 2} L${m - 4} ${m + 3} L${s - 8} ${s - 4}" stroke="${c}" stroke-width="2.5" fill="none" stroke-linejoin="round"/>`;
    default:
      return '';
  }
}

export const WHOLE_GLYPH = new Set<ToothState>(['endodoncia', 'corona', 'extraccion_indicada', 'ausente', 'implante', 'protesis']);

export function emptyOdontogram(dentition: Dentition = 'adult'): OdontogramData {
  return { dentition, teeth: {} };
}

/** Dentition that fits an age: baby teeth until ~6, both sets while they are replaced (~6-12), permanent after. */
export function dentitionForAge(age: number | null | undefined): Dentition {
  if (age == null) return 'adult';
  return age < 6 ? 'child' : age < 13 ? 'mixed' : 'adult';
}

/** The age-based dentition, widened to "mixed" when recorded teeth would otherwise be hidden. */
export function autoDentition(age: number | null | undefined, data: OdontogramData): Dentition {
  const want = dentitionForAge(age);
  if (want === 'mixed') return want;
  const keys = Object.keys(data.teeth).map(Number);
  const hasBaby = keys.some((n) => n >= 51);
  const hasPerm = keys.some((n) => n < 51);
  if (want === 'adult' && hasBaby) return 'mixed';
  if (want === 'child' && hasPerm) return 'mixed';
  return want;
}

/** Removes empty teeth so the stored snapshot only holds findings. */
export function cleanTeeth(data: OdontogramData): OdontogramData {
  const teeth: Record<string, ToothData> = {};
  for (const [k, t] of Object.entries(data.teeth)) {
    const surfaces = Object.fromEntries(Object.entries(t.surfaces ?? {}).filter(([, v]) => !!v));
    const out: ToothData = {};
    if (t.state) out.state = t.state;
    if (Object.keys(surfaces).length) out.surfaces = surfaces;
    if (t.note?.trim()) out.note = t.note.trim();
    if (Object.keys(out).length) teeth[k] = out;
  }
  return { dentition: data.dentition, teeth };
}

export function toothSummary(t: ToothData | undefined): string {
  if (!t) return '';
  const parts: string[] = [];
  if (t.state) parts.push(stateDef(t.state)?.label ?? t.state);
  for (const [s, st] of Object.entries(t.surfaces ?? {})) parts.push(`${SURFACE_NAMES[s as Surface] ?? s}: ${stateDef(st)?.label ?? st}`);
  return parts.join(', ');
}

/** Teeth whose findings differ between two snapshots, with a short before/after text. */
export function diffOdontograms(prev: OdontogramData | null, cur: OdontogramData): { tooth: number; before: string; after: string }[] {
  const keys = new Set([...Object.keys(prev?.teeth ?? {}), ...Object.keys(cur.teeth)]);
  const out: { tooth: number; before: string; after: string }[] = [];
  for (const k of [...keys].sort((a, b) => Number(a) - Number(b))) {
    const a = toothSummary(prev?.teeth[k]);
    const b = toothSummary(cur.teeth[k]);
    const na = prev?.teeth[k]?.note ?? '';
    const nb = cur.teeth[k]?.note ?? '';
    if (a !== b || na !== nb) out.push({ tooth: Number(k), before: a || 'Sin hallazgos', after: b || 'Sin hallazgos' });
  }
  return out;
}

/** Static SVG (string) of an odontogram for printing. */
export function odontogramSvg(data: OdontogramData, highlight: Set<number> = new Set()): string {
  const rows = rowsFor(data.dentition);
  const w = Math.max(...rows.map(rowWidth));
  const h = rows.length * (ROW_H + 8);
  let body = '';
  rows.forEach((row, ri) => {
    const y0 = ri * (ROW_H + 8);
    row.teeth.forEach((n, i) => {
      const x = toothX(row, i);
      const t = data.teeth[String(n)];
      const label = `<text x="${x + CELL / 2}" y="${y0 + (row.arch === 'upper' ? 11 : CELL + 22)}" font-size="10" text-anchor="middle" font-family="Arial" fill="#000">${n}</text>`;
      const gy = y0 + (row.arch === 'upper' ? 15 : 3);
      let cell = `<g transform="translate(${x} ${gy})">`;
      if (highlight.has(n)) cell += `<rect x="-3" y="-3" width="${CELL + 6}" height="${CELL + 6}" rx="6" fill="none" stroke="#000" stroke-width="1.5" stroke-dasharray="3 2"/>`;
      for (const sh of surfaceShapes(n)) {
        const st = t?.surfaces?.[sh.surface];
        cell += `<polygon points="${sh.points}" fill="${st ? (stateDef(st)?.color ?? '#999') : '#fff'}" stroke="#000" stroke-width=".8"/>`;
      }
      if (t?.state) cell += glyph(t.state);
      cell += '</g>';
      body += label + cell;
    });
  });
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${w} ${h}" width="100%" style="max-width:720px">${body}</svg>`;
}
