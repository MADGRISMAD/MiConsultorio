import type { ClinicSettings } from '$lib/types';
import type { Appt, DayKey, Professional } from '$lib/types/agenda';
import { PRO_COLORS } from '$lib/types/agenda';

const pad = (n: number) => String(n).padStart(2, '0');

/** Local date as YYYY-MM-DD (never through UTC, which shifts the day at night in Mexico). */
export const ymd = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
export const parseDay = (s: string) => new Date(s + 'T12:00:00');
export const todayStr = () => ymd(new Date());
export function addDays(s: string, n: number): string {
  const d = parseDay(s);
  d.setDate(d.getDate() + n);
  return ymd(d);
}
export function addMonths(s: string, n: number): string {
  const d = parseDay(s);
  d.setDate(1);
  d.setMonth(d.getMonth() + n);
  return ymd(d);
}
export function weekStart(s: string): string {
  const d = parseDay(s);
  return addDays(s, -((d.getDay() + 6) % 7));
}
/** The six weeks that cover the month of s, starting on Monday. */
export function monthDays(s: string): string[] {
  const first = s.slice(0, 8) + '01';
  const start = weekStart(first);
  return Array.from({ length: 42 }, (_, i) => addDays(start, i));
}

export const toMin = (t: string) => {
  const [h, m] = t.split(':').map(Number);
  return h * 60 + m;
};
export const fromMin = (n: number) => `${pad(Math.floor(n / 60))}:${pad(n % 60)}`;

const DAY_KEYS: DayKey[] = ['sun', 'mon', 'tue', 'wed', 'thu', 'fri', 'sat'];
export const dayKey = (s: string): DayKey => DAY_KEYS[parseDay(s).getDay()];

const cap = (t: string) => t.charAt(0).toUpperCase() + t.slice(1);
export const fmtLong = (s: string) => cap(longFmt(s));
const longFmt = (s: string) => parseDay(s).toLocaleDateString('es-MX', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' });
export const fmtShort = (s: string) => cap(parseDay(s).toLocaleDateString('es-MX', { weekday: 'short', day: 'numeric', month: 'short' }));
export const fmtMonth = (s: string) => cap(parseDay(s).toLocaleDateString('es-MX', { month: 'long', year: 'numeric' }));
export const WEEKDAY_SHORT = ['Lun', 'Mar', 'Mié', 'Jue', 'Vie', 'Sáb', 'Dom'];

export function rangeTitle(view: string, cursor: string): string {
  if (view === 'day') return fmtLong(cursor);
  if (view === 'week') {
    const a = weekStart(cursor), b = addDays(a, 6);
    const f = (s: string, y: boolean) => parseDay(s).toLocaleDateString('es-MX', { day: 'numeric', month: 'short', ...(y ? { year: 'numeric' } : {}) });
    return `${f(a, false)} – ${f(b, true)}`;
  }
  return fmtMonth(cursor);
}

/** Visible hour range (in minutes) from the clinic's opening hours over the given days, widened to hold every appointment. */
export function visibleRange(clinic: ClinicSettings | undefined, days: string[], appts: Appt[]): [number, number] {
  let lo = Infinity, hi = -Infinity;
  for (const d of days) {
    const h = clinic?.hours?.[dayKey(d)];
    if (h?.open) {
      lo = Math.min(lo, toMin(h.start));
      hi = Math.max(hi, toMin(h.end));
    }
  }
  if (!isFinite(lo)) [lo, hi] = [8 * 60, 20 * 60];
  for (const a of appts) {
    lo = Math.min(lo, toMin(a.startHour));
    hi = Math.max(hi, toMin(a.endHour));
  }
  lo = Math.max(0, Math.floor(lo / 60) * 60 - 0);
  hi = Math.min(24 * 60, Math.ceil(hi / 60) * 60);
  if (hi - lo < 4 * 60) hi = Math.min(24 * 60, lo + 4 * 60);
  return [lo, hi];
}

export function proColor(pros: Professional[], id: string | null): string {
  if (!id) return '#8a97a6';
  const i = pros.findIndex((p) => p.id === id);
  if (i < 0) return '#8a97a6';
  return pros[i].color || PRO_COLORS[i % PRO_COLORS.length];
}

export const fullName = (a: { names: string; last_names: string }) => `${a.names} ${a.last_names}`.trim();

/** Places overlapping appointments side by side: returns [lane, lanes] per appointment. */
export function layoutLanes(items: Appt[]): Map<string, [number, number]> {
  const sorted = [...items].sort((a, b) => toMin(a.startHour) - toMin(b.startHour) || toMin(b.endHour) - toMin(a.endHour));
  const out = new Map<string, [number, number]>();
  let group: Appt[] = [];
  let groupEnd = -1;
  const flush = () => {
    const laneEnds: number[] = [];
    const lane = new Map<string, number>();
    for (const a of group) {
      let l = laneEnds.findIndex((e) => e <= toMin(a.startHour));
      if (l < 0) l = laneEnds.length;
      laneEnds[l] = toMin(a.endHour);
      lane.set(a.id, l);
    }
    for (const a of group) out.set(a.id, [lane.get(a.id)!, laneEnds.length]);
    group = [];
  };
  for (const a of sorted) {
    if (group.length && toMin(a.startHour) >= groupEnd) flush();
    group.push(a);
    groupEnd = Math.max(groupEnd, toMin(a.endHour));
    if (group.length === 1) groupEnd = toMin(a.endHour);
  }
  if (group.length) flush();
  return out;
}
