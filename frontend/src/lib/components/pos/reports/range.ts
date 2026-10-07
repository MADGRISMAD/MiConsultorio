/** Local calendar day as YYYY-MM-DD (what the API expects). */
export const ymd = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;

export type Preset = 'today' | 'yesterday' | '7d' | 'month' | 'lastmonth' | 'custom';

export const PRESETS: { id: Preset; label: string }[] = [
  { id: 'today', label: 'Hoy' },
  { id: 'yesterday', label: 'Ayer' },
  { id: '7d', label: '7 días' },
  { id: 'month', label: 'Este mes' },
  { id: 'lastmonth', label: 'Mes pasado' },
  { id: 'custom', label: 'Personalizado' }
];

export function presetRange(p: Preset, now = new Date()): { from: string; to: string } {
  const day = (offset: number) => new Date(now.getFullYear(), now.getMonth(), now.getDate() + offset);
  switch (p) {
    case 'yesterday':
      return { from: ymd(day(-1)), to: ymd(day(-1)) };
    case '7d':
      return { from: ymd(day(-6)), to: ymd(day(0)) };
    case 'month':
      return { from: ymd(new Date(now.getFullYear(), now.getMonth(), 1)), to: ymd(day(0)) };
    case 'lastmonth':
      return { from: ymd(new Date(now.getFullYear(), now.getMonth() - 1, 1)), to: ymd(new Date(now.getFullYear(), now.getMonth(), 0)) };
    default:
      return { from: ymd(day(0)), to: ymd(day(0)) };
  }
}
