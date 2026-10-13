// Calendar days as YYYY-MM-DD, always in local time: going through toISOString() would be UTC, and in Mexico after about
// 18:00 that is already tomorrow.

const pad = (n: number) => String(n).padStart(2, '0');

/** Local date as YYYY-MM-DD. */
export const ymd = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
/** A YYYY-MM-DD string as a Date at local noon (so no hour change moves the day). */
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
