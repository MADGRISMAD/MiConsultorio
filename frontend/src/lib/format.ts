const rtf = typeof Intl !== 'undefined' ? new Intl.RelativeTimeFormat('es', { numeric: 'auto' }) : null;

/** "hace 3 días", "ayer", "en 2 horas"… */
export function ago(iso: string | null | undefined): string {
  if (!iso) return 'nunca';
  const diff = (new Date(iso).getTime() - Date.now()) / 1000;
  const steps: [number, Intl.RelativeTimeFormatUnit][] = [
    [60, 'second'],
    [60, 'minute'],
    [24, 'hour'],
    [7, 'day'],
    [4.345, 'week'],
    [12, 'month'],
    [Infinity, 'year']
  ];
  let value = diff;
  for (const [size, unit] of steps) {
    if (Math.abs(value) < size) return rtf!.format(Math.round(value), unit);
    value /= size;
  }
  return '';
}

/** A calendar day without the shift of the time zone: «2026-10-09» is read at local noon, not as UTC midnight (the day before in Mexico). */
const asDate = (iso: string) => (iso.length === 10 ? new Date(iso + 'T12:00:00') : new Date(iso));

export const dateShort = (iso: string | null | undefined) =>
  iso ? asDate(iso).toLocaleDateString('es-MX', { day: 'numeric', month: 'short', year: 'numeric' }) : '—';

const DATE_TIME: Intl.DateTimeFormatOptions = { day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false };

/** "9 oct 2026, 14:30" */
export const dateTime = (iso: string) => new Date(iso).toLocaleString('es-MX', DATE_TIME);
/** "vie, 9 oct 2026, 14:30" */
export const dateTimeWeekday = (iso: string) => new Date(iso).toLocaleString('es-MX', { weekday: 'short', ...DATE_TIME });

/** «3 años», «1 año», or «5 meses» for a baby under one year (counted from the birth date). Empty when the age is unknown. */
export function patientAge(p: { age: number | null | undefined; birth_date?: string | null }): string {
  if (p.age == null) return '';
  if (p.age < 1 && p.birth_date) {
    const b = new Date(p.birth_date + 'T12:00:00');
    const now = new Date();
    const m = Math.max(0, (now.getFullYear() - b.getFullYear()) * 12 + now.getMonth() - b.getMonth());
    return `${m} ${m === 1 ? 'mes' : 'meses'}`;
  }
  return `${p.age} ${p.age === 1 ? 'año' : 'años'}`;
}

/** "Ana Pérez" */
export const fullName = (p: { names: string; last_names: string }) => `${p.names} ${p.last_names}`.trim();

/** The one rule for an e-mail address in a form (the server checks it again). */
export const emailOk = (v: string) => /^[^\s@,;<>]+@[^\s@,;<>]+\.[^\s@,;<>]+$/.test(v.trim());

export const money = (pesos: number) => pesos.toLocaleString('es-MX', { style: 'currency', currency: 'MXN', maximumFractionDigits: 0 });
export const moneyCents = (cents: number) => (cents / 100).toLocaleString('es-MX', { style: 'currency', currency: 'MXN' });

export const initials = (name: string) =>
  name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w[0])
    .join('')
    .toUpperCase() || '?';
