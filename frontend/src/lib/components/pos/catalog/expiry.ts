/** A date-only value (YYYY-MM-DD) in local time, without the day shift Date.parse would cause. */
export function ymd(s: string): Date {
  const [y, m, d] = s.split('-').map(Number);
  return new Date(y, (m ?? 1) - 1, d ?? 1);
}

export const dayLabel = (s: string | null | undefined) =>
  s ? ymd(s).toLocaleDateString('es-MX', { day: '2-digit', month: 'short', year: 'numeric' }).replace('.', '') : '—';

function daysLeft(s: string): number {
  const t = new Date();
  const today = new Date(t.getFullYear(), t.getMonth(), t.getDate());
  return Math.round((ymd(s).getTime() - today.getTime()) / 86_400_000);
}

/** Pill tone for an expiry date: red when expired, amber within 60 days. */
export function expiryTone(s: string | null | undefined): 'bad' | 'warn' | 'muted' {
  if (!s) return 'muted';
  const d = daysLeft(s);
  return d < 0 ? 'bad' : d <= 60 ? 'warn' : 'muted';
}
