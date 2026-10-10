import { patientPath, request, seg } from '$lib/api';
import type { GrowthCoverage, GrowthImportInput, GrowthImportPreview, GrowthImportRecord, LabCatalog, LabOrder, LabOrderInput, LabResultInput, LabTrends, PatientGrowth } from '$lib/types/lab';

const p = patientPath;

export const labApi = {
  catalog: () => request<LabCatalog>('GET', '/lab/catalog'),
  orders: (patientId: string) => request<{ orders: LabOrder[] }>('GET', `${p(patientId)}/lab/orders`).then((r) => r.orders),
  createOrder: (patientId: string, o: LabOrderInput) => request<{ order: LabOrder }>('POST', `${p(patientId)}/lab/orders`, o).then((r) => r.order),
  updateOrder: (id: string, o: { title: string; lab_name: string; notes: string; attachment_id: string }) =>
    request<{ order: LabOrder }>('PUT', `/lab/orders/${seg(id)}`, o).then((r) => r.order),
  setStatus: (id: string, status: 'parcial' | 'completo' | 'cancelado', reason = '') =>
    request<{ order: LabOrder }>('POST', `/lab/orders/${seg(id)}/status`, { status, reason }).then((r) => r.order),
  addResults: (id: string, results: LabResultInput[], complete = false) =>
    request<{ order: LabOrder }>('POST', `/lab/orders/${seg(id)}/results`, { results, complete }).then((r) => r.order),
  trends: (patientId: string, analyte = '') => request<LabTrends>('GET', `${p(patientId)}/lab-trends?analyte=${seg(analyte)}`)
};

export const growthApi = {
  patient: (patientId: string, standard = '') => request<PatientGrowth>('GET', `${p(patientId)}/growth${standard ? `?standard=${seg(standard)}` : ''}`),
  imports: () => request<{ imports: GrowthImportRecord[] }>('GET', '/growth/references').then((r) => r.imports),
  /** what one table covers: rows and age range per indicator and sex */
  detail: (id: string) => request<{ import: GrowthImportRecord; groups: GrowthCoverage[] }>('GET', `/growth/references/${encodeURIComponent(id)}`),
  preview: (b: Omit<GrowthImportInput, 'confirm'>) =>
    request<{ preview: GrowthImportPreview }>('POST', '/growth/references/import', { ...b, confirm: false }).then((r) => r.preview),
  confirm: (b: Omit<GrowthImportInput, 'confirm'>) =>
    request<{ import: GrowthImportRecord }>('POST', '/growth/references/import', { ...b, confirm: true }).then((r) => r.import)
};
