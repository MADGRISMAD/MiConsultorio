import { ApiError, request, seg } from '$lib/api';
import type { Prescription } from '$lib/types';
import type { CatalogMed, ChronicMed, ConsultSummaryResult, ClinicMedInput, Icd10, RxCheck, RxCreateResult, RxInput, RxPrintData, RxVerification } from '$lib/types/rx';

export const rxApi = {
  medications: (q: string, subject: 'person' | 'animal', species = '', limit = 20) =>
    request<{ medications: CatalogMed[]; disclaimer: string }>(
      'GET',
      `/rx/medications?q=${seg(q)}&subject=${subject}${species ? `&species=${seg(species)}` : ''}&limit=${limit}`
    ),
  diagnoses: (q: string, limit = 15, subject: 'person' | 'animal' = 'person', species = '') =>
    request<{ diagnoses: Icd10[]; disclaimer: string }>(
      'GET',
      `/rx/diagnoses?q=${seg(q)}&limit=${limit}${subject === 'animal' ? `&subject=animal${species ? `&species=${seg(species)}` : ''}` : ''}`
    ),

  clinicMeds: () => request<{ medications: CatalogMed[] }>('GET', '/rx/clinic-medications').then((r) => r.medications),
  saveClinicMed: (m: ClinicMedInput, id?: string) =>
    request<{ medication: CatalogMed }>(id ? 'PUT' : 'POST', id ? `/rx/clinic-medications/${seg(id)}` : '/rx/clinic-medications', m).then((r) => r.medication),
  deleteClinicMed: (id: string) => request<{ ok: boolean }>('DELETE', `/rx/clinic-medications/${seg(id)}`),

  /** The alerts of a receta while it is being written: allergies, interactions and what the patient already takes. */
  check: (patientId: string, items: { medicine: string; brand?: string }[]) => request<RxCheck>('POST', `/patients/${seg(patientId)}/rx-check`, { items }),

  chronic: {
    list: (patientId: string) => request<{ medications: ChronicMed[] }>('GET', `/patients/${seg(patientId)}/chronic-meds`).then((r) => r.medications),
    add: (patientId: string, m: Partial<ChronicMed>) =>
      request<{ medication: ChronicMed; alerts: { allergies: RxCheck['allergies']; interactions: RxCheck['interactions'] } }>('POST', `/patients/${seg(patientId)}/chronic-meds`, m),
    update: (patientId: string, id: string, m: Partial<ChronicMed> & { stop?: boolean }) =>
      request<{ medication: ChronicMed }>('PATCH', `/patients/${seg(patientId)}/chronic-meds/${seg(id)}`, m).then((r) => r.medication)
  },

  /** An AI summary of the record for the specialist about to see the patient; costs one magic use. */
  summary: (patientId: string) => request<ConsultSummaryResult>('POST', `/patients/${seg(patientId)}/ai-summary`, {}),

  printData: (id: string) => request<RxPrintData>('GET', `/prescriptions/${seg(id)}`),
  verify: (token: string) => request<RxVerification>('GET', `/public/rx/${seg(token)}`),

  /** Like api.patients.createPrescription, but allergy and dose warnings come back as data to be confirmed. */
  async createPrescription(patientId: string, body: RxInput): Promise<RxCreateResult> {
    let res: Response;
    try {
      res = await fetch(`/api/patients/${seg(patientId)}/prescriptions`, {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
      });
    } catch {
      throw new ApiError('No se pudo conectar con el servidor.', 0);
    }
    const data = await res.json().catch(() => null);
    if (res.ok) return { kind: 'created', prescription: data.prescription as Prescription, interactions: data.interactions ?? [] };
    if (res.status === 409 && data?.code === 'INTERACTION_CONFLICT') return { kind: 'interaction', message: data.message, interactions: data.interactions ?? [] };
    if (res.status === 409 && data?.code === 'ALLERGY_CONFLICT') return { kind: 'allergy', message: data.message, conflicts: data.conflicts ?? [] };
    if (res.status === 409 && data?.code === 'DOSE_WARNING') return { kind: 'dose', message: data.message, warnings: data.warnings ?? [] };
    throw new ApiError(data?.message ?? 'Ocurrió un error inesperado.', res.status, data?.code ?? '');
  }
};
