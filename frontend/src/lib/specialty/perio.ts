import type { PerioData, PerioTooth } from '$lib/types/specialty';

/** Permanent teeth in the order the dentist reads them: the upper arch from the right molar, then the lower. */
export const UPPER = ['18', '17', '16', '15', '14', '13', '12', '11', '21', '22', '23', '24', '25', '26', '27', '28'];
export const LOWER = ['48', '47', '46', '45', '44', '43', '42', '41', '31', '32', '33', '34', '35', '36', '37', '38'];
export const ALL = [...UPPER, ...LOWER];
export const SITES = ['Distal', 'Medio', 'Mesial'];

export const emptyTooth = (): PerioTooth => ({ pd: Array(6).fill(null), rec: Array(6).fill(null), bop: Array(6).fill(false), mob: 0, furc: 0, plaque: false });
export const emptyPerio = (): PerioData => ({ teeth: {}, notes: '', plaque_pct: 0, bleeding_pct: 0 });

/** Colour of a probing depth: up to 3 mm is healthy, 4 mm needs watching, 5 mm or more is a periodontal pocket. */
export function pdTone(v: number | null | undefined): string {
  if (v == null) return 'bg-app-ink/5 text-app-muted';
  if (v <= 3) return 'bg-app-accent/15 text-app-ink';
  if (v === 4) return 'bg-app-warning/25 text-app-ink';
  return 'bg-app-danger/20 text-app-ink';
}

export interface PerioStats {
  teeth: number;
  sites: number;
  bleedingSites: number;
  bleedingPct: number;
  plaquePct: number;
  pockets4: number;
  pockets5: number;
  meanPd: number;
  meanCal: number;
}

/** Totals of a chart. The percentages follow the usual indices: sites that bled over sites probed, and teeth with plaque over present teeth. */
export function perioStats(d: PerioData): PerioStats {
  let teeth = 0, sites = 0, bleeding = 0, plaque = 0, p4 = 0, p5 = 0, sumPd = 0, sumCal = 0;
  for (const t of Object.values(d.teeth)) {
    if (t.absent) continue;
    teeth++;
    if (t.plaque) plaque++;
    for (let i = 0; i < 6; i++) {
      const pd = t.pd[i];
      if (pd == null) continue;
      sites++;
      sumPd += pd;
      sumCal += pd + (t.rec[i] ?? 0);
      if (t.bop[i]) bleeding++;
      if (pd === 4) p4++;
      else if (pd >= 5) p5++;
    }
  }
  const pct = (a: number, b: number) => (b ? Math.round((a / b) * 1000) / 10 : 0);
  return {
    teeth,
    sites,
    bleedingSites: bleeding,
    bleedingPct: pct(bleeding, sites),
    plaquePct: pct(plaque, teeth),
    pockets4: p4,
    pockets5: p5,
    meanPd: sites ? Math.round((sumPd / sites) * 10) / 10 : 0,
    meanCal: sites ? Math.round((sumCal / sites) * 10) / 10 : 0
  };
}

export const maxPd = (t: PerioTooth | undefined) => (t && !t.absent ? t.pd.reduce<number | null>((m, v) => (v == null ? m : m == null || v > m ? v : m), null) : null);

/** A chart that holds nothing but empty teeth is not worth saving. */
export const hasPerioData = (d: PerioData) => Object.values(d.teeth).some((t) => t.absent || t.pd.some((v) => v != null) || t.plaque || t.mob > 0 || t.furc > 0);

/** A tooth with nothing measured is not stored. */
export function cleanPerio(d: PerioData): PerioData {
  const teeth: Record<string, PerioTooth> = {};
  for (const k of Object.keys(d.teeth)) {
    const t = d.teeth[k];
    if (t.absent) teeth[k] = { ...emptyTooth(), absent: true };
    else if (t.pd.some((v) => v != null) || t.rec.some((v) => v != null) || t.bop.some(Boolean) || t.plaque || t.mob > 0 || t.furc > 0) teeth[k] = JSON.parse(JSON.stringify(t));
  }
  const st = perioStats({ ...d, teeth });
  return { teeth, notes: d.notes.trim(), plaque_pct: st.plaquePct, bleeding_pct: st.bleedingPct };
}
