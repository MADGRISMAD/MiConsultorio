import type { Encounter, FieldDef, FieldValues } from '$lib/types';

/** Display text of a captured value, '' when empty. */
export function show(f: FieldDef, values: FieldValues | undefined): string {
  const v = values?.[f.key];
  if (v == null || v === '' || (Array.isArray(v) && !v.length)) return '';
  if (Array.isArray(v)) return v.join(', ');
  if (typeof v === 'boolean') return v ? 'Sí' : 'No';
  if (f.type === 'date' && typeof v === 'string') return new Date(`${v}T12:00:00`).toLocaleDateString('es-MX', { day: 'numeric', month: 'long', year: 'numeric' });
  return String(v) + (f.unit ? ` ${f.unit}` : '');
}

export function measureRows(defs: FieldDef[], values: Encounter['measures']) {
  return defs.map((d) => ({ label: d.label, value: show(d, values) })).filter((r) => r.value);
}
