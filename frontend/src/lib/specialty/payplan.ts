import type { PaymentPlan } from '$lib/types/specialty';

export interface PayRow {
  label: string;
  /** YYYY-MM-DD; the down payment is due when the plan is accepted */
  date: string | null;
  cents: number;
}

const addDays = (ymd: string, n: number) => {
  const d = new Date(`${ymd}T12:00:00`);
  d.setDate(d.getDate() + n);
  return d.toLocaleDateString('en-CA');
};

/** The payments of a plan: the down payment and equal installments; the last one absorbs the cents left over. */
export function paySchedule(total: number, pp: PaymentPlan): PayRow[] {
  const down = Math.round((total * pp.down_pct) / 100);
  const rest = total - down;
  const base = Math.floor(rest / pp.installments);
  const rows: PayRow[] = [];
  if (down > 0) rows.push({ label: 'Enganche', date: null, cents: down });
  for (let i = 0; i < pp.installments; i++) {
    const cents = i === pp.installments - 1 ? rest - base * (pp.installments - 1) : base;
    rows.push({ label: pp.installments === 1 ? 'Pago único' : `Pago ${i + 1} de ${pp.installments}`, date: addDays(pp.start_date, i * pp.interval_days), cents });
  }
  return rows;
}

export const defaultPayPlan = (): PaymentPlan => ({ down_pct: 30, installments: 6, interval_days: 30, start_date: addDays(new Date().toLocaleDateString('en-CA'), 30) });
