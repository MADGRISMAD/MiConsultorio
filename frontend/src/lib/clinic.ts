import { WEEKDAYS, type ClinicSettings, type DayHours, type Weekday } from './types';

export const DURATIONS = [15, 20, 30, 45, 60, 90];

/** "Lun–Vie 09:00–18:00 · Sáb 09:00–13:00" — consecutive days with the same hours are grouped. */
export function summarizeHours(settings: ClinicSettings): string {
  const short: Record<Weekday, string> = { mon: 'Lun', tue: 'Mar', wed: 'Mié', thu: 'Jue', fri: 'Vie', sat: 'Sáb', sun: 'Dom' };
  const groups: { from: Weekday; to: Weekday; h: DayHours }[] = [];
  for (const [key] of WEEKDAYS) {
    const h = settings.hours[key];
    if (!h.open) continue;
    const last = groups[groups.length - 1];
    const prev = last && WEEKDAYS[WEEKDAYS.findIndex(([k]) => k === key) - 1]?.[0] === last.to;
    if (last && prev && last.h.start === h.start && last.h.end === h.end) last.to = key;
    else groups.push({ from: key, to: key, h });
  }
  if (groups.length === 0) return 'Sin días de atención';
  return groups.map((g) => `${g.from === g.to ? short[g.from] : `${short[g.from]}–${short[g.to]}`} ${g.h.start}–${g.h.end}`).join(' · ');
}

export function addMinutes(hhmm: string, minutes: number): string {
  const [h, m] = hhmm.split(':').map(Number);
  const total = Math.min(h * 60 + m + minutes, 23 * 60 + 59);
  return `${String(Math.floor(total / 60)).padStart(2, '0')}:${String(total % 60).padStart(2, '0')}`;
}

const DOW: Weekday[] = ['sun', 'mon', 'tue', 'wed', 'thu', 'fri', 'sat'];

/** A short warning when an appointment falls outside the clinic's opening hours, else ''. */
export function outsideHours(settings: ClinicSettings | undefined, date: string, start: string, end: string): string {
  if (!settings || !date) return '';
  const day = DOW[new Date(date + 'T12:00:00').getDay()];
  const h = settings.hours[day];
  if (!h) return '';
  if (!h.open) return 'El consultorio no atiende ese día según tu horario.';
  if ((start && start < h.start) || (end && end > h.end)) return `Fuera del horario de atención (${h.start}–${h.end}).`;
  return '';
}
