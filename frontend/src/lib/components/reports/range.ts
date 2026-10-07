import { ymd } from '$lib/components/pos/reports/range';

export type RangePreset = 'today' | '7d' | '30d' | '90d' | 'month' | 'custom';

export const RANGE_PRESETS: { id: RangePreset; label: string }[] = [
  { id: 'today', label: 'Hoy' },
  { id: '7d', label: '7 días' },
  { id: '30d', label: '30 días' },
  { id: '90d', label: '90 días' },
  { id: 'month', label: 'Mes actual' },
  { id: 'custom', label: 'Personalizado' }
];

export function rangeFor(p: RangePreset, now = new Date()): { from: string; to: string } {
  const day = (offset: number) => new Date(now.getFullYear(), now.getMonth(), now.getDate() + offset);
  switch (p) {
    case '7d':
      return { from: ymd(day(-6)), to: ymd(day(0)) };
    case '30d':
      return { from: ymd(day(-29)), to: ymd(day(0)) };
    case '90d':
      return { from: ymd(day(-89)), to: ymd(day(0)) };
    case 'month':
      return { from: ymd(new Date(now.getFullYear(), now.getMonth(), 1)), to: ymd(day(0)) };
    default:
      return { from: ymd(day(0)), to: ymd(day(0)) };
  }
}

/** Days in an inclusive YYYY-MM-DD range. */
export function rangeDays(from: string, to: string): number {
  const a = new Date(from + 'T12:00:00').getTime();
  const b = new Date(to + 'T12:00:00').getTime();
  return Math.round((b - a) / 86400000) + 1;
}

export const MAX_RANGE_DAYS = 731;
