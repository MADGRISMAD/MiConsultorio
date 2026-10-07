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
