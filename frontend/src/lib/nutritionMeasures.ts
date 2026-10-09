import type { Encounter } from '$lib/types';

/** The body measures a nutritionist follows, in the order of the progress table. */
export const BODY_MEASURES: { key: string; label: string; short: string; unit: string; lowerIsBetter?: boolean }[] = [
  { key: 'weight_kg', label: 'Peso', short: 'Peso', unit: 'kg', lowerIsBetter: true },
  { key: 'height_cm', label: 'Talla', short: 'Talla', unit: 'cm' },
  { key: 'waist_cm', label: 'Cintura', short: 'Cintura', unit: 'cm', lowerIsBetter: true },
  { key: 'hip_cm', label: 'Cadera', short: 'Cadera', unit: 'cm' },
  { key: 'arm_cm', label: 'Brazo', short: 'Brazo', unit: 'cm' },
  { key: 'body_fat', label: 'Grasa corporal', short: '% grasa', unit: '%', lowerIsBetter: true },
  { key: 'muscle_kg', label: 'Masa muscular', short: 'Músculo', unit: 'kg' },
  { key: 'visceral_fat', label: 'Grasa visceral', short: 'G. visceral', unit: '', lowerIsBetter: true },
  { key: 'body_water', label: 'Agua corporal', short: '% agua', unit: '%' }
];

export const numOf = (v: unknown): number => {
  const n = Number(String(v ?? '').replace(',', '.'));
  return Number.isFinite(n) && n > 0 ? n : 0;
};

export const bmiOf = (weight: number, heightCm: number): number | null => (weight > 0 && heightCm > 30 ? Math.round((weight / (heightCm / 100) ** 2) * 10) / 10 : null);
export const bmiLabel = (v: number) => (v < 18.5 ? 'bajo peso' : v < 25 ? 'peso normal' : v < 30 ? 'sobrepeso' : 'obesidad');

/** the visible measurements, oldest first */
export const measureRows = (encounters: Encounter[]) =>
  encounters
    .filter((e) => !e.hidden && BODY_MEASURES.some((m) => numOf(e.measures?.[m.key]) > 0))
    .sort((a, b) => a.occurred_at.localeCompare(b.occurred_at));

/** the most recent value of each measure, newest note first */
export function latestMeasures(encounters: Encounter[]): Record<string, string> {
  const out: Record<string, string> = {};
  for (const e of [...measureRows(encounters)].reverse())
    for (const m of BODY_MEASURES) if (!out[m.key] && numOf(e.measures?.[m.key]) > 0) out[m.key] = String(e.measures[m.key]);
  return out;
}
