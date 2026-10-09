import type { BodyFinding, BodymapData, BodyView, FindingKind } from '$lib/types/specialty';

export const KINDS: { id: FindingKind; label: string; letter: string }[] = [
  { id: 'dolor', label: 'Dolor', letter: 'D' },
  { id: 'contractura', label: 'Contractura', letter: 'C' },
  { id: 'subluxacion', label: 'Subluxación', letter: 'S' },
  { id: 'parestesia', label: 'Parestesia', letter: 'P' },
  { id: 'otro', label: 'Otro', letter: 'O' }
];
export const kindLabel = (k: string) => KINDS.find((x) => x.id === k)?.label ?? k;

export interface Zone {
  id: string;
  label: string;
  kind: 'rect' | 'ellipse';
  x: number; // rect: left; ellipse: center
  y: number;
  w: number; // rect width / ellipse rx
  h: number; // rect height / ellipse ry
}

export const VIEW_W = 200;
export const VIEW_H = 410;

type Def = [id: string, label: string, kind: 'rect' | 'ellipse', x: number, y: number, w: number, h: number, mirror?: boolean];

// Geometry drawn for the patient's left side as it appears in the front view (viewer's right).
const FRONT: Def[] = [
  ['head', 'Cabeza', 'ellipse', 100, 22, 19, 18],
  ['face', 'Cara', 'ellipse', 100, 46, 14, 11],
  ['neck', 'Cuello', 'rect', 91, 60, 18, 16],
  ['chest', 'Tórax', 'rect', 72, 80, 56, 46],
  ['abdomen', 'Abdomen', 'rect', 74, 128, 52, 44],
  ['groin', 'Ingle / pelvis', 'rect', 88, 174, 24, 26],
  ['shoulder', 'Hombro', 'ellipse', 140, 90, 16, 12, true],
  ['upper_arm', 'Brazo', 'rect', 146, 104, 16, 44, true],
  ['elbow', 'Codo', 'ellipse', 154, 154, 10, 8, true],
  ['forearm', 'Antebrazo', 'rect', 146, 164, 16, 40, true],
  ['hand', 'Mano y muñeca', 'ellipse', 154, 220, 10, 15, true],
  ['hip', 'Cadera', 'ellipse', 124, 186, 12, 15, true],
  ['thigh', 'Muslo', 'rect', 102, 204, 26, 72, true],
  ['knee', 'Rodilla', 'ellipse', 115, 286, 13, 10, true],
  ['shin', 'Pierna (espinilla)', 'rect', 103, 298, 24, 66, true],
  ['foot', 'Pie', 'ellipse', 114, 380, 14, 10, true]
];

const BACK: Def[] = [
  ['head', 'Cabeza (nuca)', 'ellipse', 100, 28, 19, 22],
  ['cervical', 'Cervical', 'rect', 91, 60, 18, 18],
  ['thoracic', 'Dorsal', 'rect', 72, 82, 56, 48],
  ['lumbar', 'Lumbar', 'rect', 74, 132, 52, 36],
  ['sacrum', 'Sacro', 'rect', 88, 170, 24, 24],
  ['shoulder', 'Hombro', 'ellipse', 140, 90, 16, 12, true],
  ['upper_arm', 'Brazo', 'rect', 146, 104, 16, 44, true],
  ['elbow', 'Codo', 'ellipse', 154, 154, 10, 8, true],
  ['forearm', 'Antebrazo', 'rect', 146, 164, 16, 40, true],
  ['hand', 'Mano y muñeca', 'ellipse', 154, 220, 10, 15, true],
  ['glute', 'Glúteo', 'ellipse', 124, 196, 16, 18, true],
  ['thigh', 'Muslo', 'rect', 102, 218, 26, 62, true],
  ['knee', 'Rodilla (hueco poplíteo)', 'ellipse', 115, 290, 13, 10, true],
  ['calf', 'Pantorrilla', 'rect', 103, 302, 24, 62, true],
  ['foot', 'Pie / talón', 'ellipse', 114, 380, 14, 10, true]
];

/**
 * Zones of a view. Paired zones come as `<name>_l` / `<name>_r` for the patient's left and right: in the
 * front view the patient's left is on the viewer's right, in the back view it is on the viewer's left.
 */
export function zonesFor(view: BodyView): Zone[] {
  const out: Zone[] = [];
  for (const [id, label, kind, x, y, w, h, pair] of view === 'front' ? FRONT : BACK) {
    if (!pair) {
      out.push({ id, label, kind, x, y, w, h });
      continue;
    }
    const mirrored = (z: { x: number; w: number }) => (kind === 'rect' ? VIEW_W - z.x - z.w : VIEW_W - z.x);
    const leftX = view === 'front' ? x : mirrored({ x, w });
    const rightX = view === 'front' ? mirrored({ x, w }) : x;
    out.push({ id: `${id}_l`, label: `${label} izquierdo`, kind, x: leftX, y, w, h });
    out.push({ id: `${id}_r`, label: `${label} derecho`, kind, x: rightX, y, w, h });
  }
  return out;
}

export const zoneLabel = (view: BodyView, id: string) => zonesFor(view).find((z) => z.id === id)?.label ?? id;

export function center(z: Zone): [number, number] {
  return z.kind === 'rect' ? [z.x + z.w / 2, z.y + z.h / 2] : [z.x, z.y];
}

/** Light yellow to deep red by intensity 0..10. */
export function intensityColor(n: number): string {
  const t = Math.max(0, Math.min(10, n)) / 10;
  const hue = 55 - 55 * t;
  const light = 62 - 18 * t;
  return `hsl(${hue.toFixed(0)} 90% ${light.toFixed(0)}%)`;
}

export function worstByZone(findings: BodyFinding[], view: BodyView): Map<string, BodyFinding> {
  const m = new Map<string, BodyFinding>();
  for (const f of findings) {
    if (f.view !== view) continue;
    const cur = m.get(f.zone);
    if (!cur || f.intensity > cur.intensity) m.set(f.zone, f);
  }
  return m;
}

export function bodyDiff(prev: BodymapData | null, cur: BodymapData): string[] {
  const key = (f: BodyFinding) => `${f.view}|${f.zone}|${f.kind}`;
  const a = new Map((prev?.zones ?? []).map((f) => [key(f), f]));
  const b = new Map(cur.zones.map((f) => [key(f), f]));
  const out: string[] = [];
  for (const [k, f] of b) {
    const o = a.get(k);
    const name = `${zoneLabel(f.view, f.zone)} (${kindLabel(f.kind).toLowerCase()})`;
    if (!o) out.push(`Nuevo: ${name}, intensidad ${f.intensity}`);
    else if (o.intensity !== f.intensity) out.push(`${name}: intensidad ${o.intensity} → ${f.intensity}`);
  }
  for (const [k, f] of a) if (!b.has(k)) out.push(`Resuelto: ${zoneLabel(f.view, f.zone)} (${kindLabel(f.kind).toLowerCase()})`);
  return out;
}

/** Static SVG string of one view, for printing. */
export function bodySvg(view: BodyView, findings: BodyFinding[]): string {
  const worst = worstByZone(findings, view);
  let body = '';
  for (const z of zonesFor(view)) {
    const f = worst.get(z.id);
    const fill = f ? intensityColor(f.intensity) : '#fff';
    const shape =
      z.kind === 'rect'
        ? `<rect x="${z.x}" y="${z.y}" width="${z.w}" height="${z.h}" rx="6" fill="${fill}" stroke="#000" stroke-width=".8"/>`
        : `<ellipse cx="${z.x}" cy="${z.y}" rx="${z.w}" ry="${z.h}" fill="${fill}" stroke="#000" stroke-width=".8"/>`;
    const [cx, cy] = center(z);
    body += shape + (f ? `<text x="${cx}" y="${cy + 4}" font-size="11" font-weight="700" text-anchor="middle" font-family="Arial">${f.intensity}</text>` : '');
  }
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${VIEW_W} ${VIEW_H}" width="200" height="410">${body}</svg>`;
}
