import type { LabAnalyteDef, LabBound, LabFlag } from '$lib/types/lab';

export const FLAG_LABEL: Record<LabFlag, string> = {
  normal: 'Normal',
  bajo: 'Bajo',
  alto: 'Alto',
  critico: 'Crítico',
  anormal: 'Alterado',
  na: 'Sin rango'
};

/** Short marker printed next to a value so the flag does not depend on colour alone. */
export const FLAG_MARK: Record<LabFlag, string> = { normal: '', bajo: '↓', alto: '↑', critico: '‼', anormal: '!', na: '' };

export const FLAG_TONE: Record<LabFlag, 'ok' | 'warn' | 'bad' | 'muted'> = { normal: 'ok', bajo: 'warn', alto: 'warn', critico: 'bad', anormal: 'warn', na: 'muted' };

/** Same rule as the server: it recomputes the flag on save, this one only previews it while typing. */
export function computeFlag(value: number | null, low: number | null, high: number | null): LabFlag {
  if (value == null || Number.isNaN(value)) return 'na';
  if (low == null && high == null) return 'na';
  if (low != null && value < low) return 'bajo';
  if (high != null && value > high) return 'alto';
  return 'normal';
}

export function rangeText(low: number | null, high: number | null): string {
  if (low != null && high != null) return `${low} – ${high}`;
  if (high != null) return `< ${high}`;
  if (low != null) return `> ${low}`;
  return '';
}

/** Catalog range of an analyte for the sex of the patient (men and women differ for some analytes). */
export function catalogBound(a: LabAnalyteDef, sex: string): LabBound | null {
  if (sex === 'Hombre' && a.ref_male) return a.ref_male;
  if (sex === 'Mujer' && a.ref_female) return a.ref_female;
  return a.ref ?? null;
}

export const dateFmt = (iso: string) => new Date(iso).toLocaleDateString('es-MX', { day: 'numeric', month: 'short', year: 'numeric' });

/** Parses a number typed by a person: accepts a decimal comma, empty gives null. */
export function parseNum(s: string): number | null {
  const t = s.trim().replace(',', '.');
  if (t === '') return null;
  const n = Number(t);
  return Number.isFinite(n) ? n : null;
}
