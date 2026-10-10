import { request, seg } from '$lib/api';
import type { OperationsReport, PatientsReport } from '$lib/types/reports';

const qs = (o: Record<string, string | number | undefined>) =>
  Object.entries(o)
    .filter(([, v]) => v !== undefined && v !== '')
    .map(([k, v]) => `${k}=${seg(String(v))}`)
    .join('&');

export const reportsApi = {
  operations: (from: string, to: string, professional = '') =>
    request<{ report: OperationsReport }>('GET', `/reports/operations?${qs({ from, to, professional })}`).then((r) => r.report),
  operationsCsvUrl: (from: string, to: string, professional = '') => `/api/reports/operations.csv?${qs({ from, to, professional })}`,
  patients: (from: string, to: string, inactiveDays: number, page: number) =>
    request<{ report: PatientsReport }>('GET', `/reports/patients?${qs({ from, to, inactive_days: inactiveDays, page, limit: 15 })}`).then((r) => r.report),
  patientsCsvUrl: (from: string, to: string, inactiveDays: number) => `/api/reports/patients.csv?${qs({ from, to, inactive_days: inactiveDays })}`,
  inactiveCsvUrl: (inactiveDays: number) => `/api/reports/patients/inactive.csv?${qs({ inactive_days: inactiveDays })}`
};

export interface IndicatorPeriod {
  encounters: number;
  appointments: number;
  completed: number;
  cancellation_rate: number;
  no_show_rate: number;
  avg_wait_min: number | null;
  avg_attention_min: number | null;
  occupancy_rate: number | null;
  new_patients: number;
  returning_patients: number;
  revenue_cents: number;
  sales: number;
  avg_ticket_cents: number;
  satisfaction_avg: number;
  satisfaction_responses: number;
}

export interface Indicators {
  from: string;
  to: string;
  prev_from: string;
  prev_to: string;
  days: number;
  current: IndicatorPeriod;
  previous: IndicatorPeriod;
  cobros: boolean;
  trend: { key: string; label: string; count: number }[];
  granularity: 'day' | 'week' | 'month';
  professionals: { id: string; name: string; encounters: number; appointments: number; no_show: number; completed: number; avg_attention_min: number | null }[];
}

export const indicatorsApi = {
  get: (from: string, to: string) => request<Indicators>('GET', `/indicators?${qs({ from, to })}`)
};
