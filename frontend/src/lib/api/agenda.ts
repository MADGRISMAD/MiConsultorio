import { request, seg } from '$lib/api';
import type {
  AgendaSettings, Appt, ApptFilters, ApptInput, ApptStatus, Professional, ProfessionalInput, ServiceOption, TimeBlock, TimeBlockInput
} from '$lib/types/agenda';

function qs(params: Record<string, string | undefined>): string {
  const q = Object.entries(params).filter(([, v]) => v).map(([k, v]) => `${k}=${seg(v as string)}`).join('&');
  return q ? `?${q}` : '';
}

export const agendaApi = {
  list: (f: ApptFilters = {}) => request<{ appointments: Appt[] }>('GET', `/appointments/${qs({ ...f })}`).then((r) => r.appointments),
  create: (input: ApptInput) => request<{ appointment: Appt }>('POST', '/appointments/', input).then((r) => r.appointment),
  update: (id: string, input: ApptInput) => request<{ appointment: Appt }>('PUT', `/appointments/${seg(id)}`, input).then((r) => r.appointment),
  remove: (id: string) => request<void>('DELETE', `/appointments/${seg(id)}`),
  setStatus: (id: string, status: ApptStatus, reason = '') =>
    request<{ appointment: Appt; charge?: { id: string; total_cents: number } }>('POST', `/appointments/${seg(id)}/status`, { status, reason }).then((r) => ({ ...r.appointment, charge: r.charge })),

  professionals: () => request<{ professionals: Professional[] }>('GET', '/agenda/professionals').then((r) => r.professionals),
  saveProfessional: (id: string, input: ProfessionalInput) =>
    request<{ professional: Professional }>('PUT', `/agenda/professionals/${seg(id)}`, input).then((r) => r.professional),
  /** free start times of a professional on a day (the calendar view of a follow-up) */
  freeSlots: (date: string, professional = '') =>
    request<{ slots: { start: string; end: string }[] }>('GET', `/agenda/free-slots${qs({ date, professional })}`).then((r) => r.slots),
  /** an open appointment of the patient in the same giro (what saving would refuse), or null */
  pendingCheck: (q: { patient?: string; professional?: string; exclude?: string; email?: string; phone?: string }) =>
    request<{ pending: { date: string; start: string; with: string; message: string } | null }>('GET', `/agenda/pending-check${qs(q)}`).then((r) => r.pending),
  /** why the public booking page of the clinic does not open, for the signed-in team (problem '' = nothing wrong) */
  bookingCheck: (slug: string) =>
    request<{ problem: string; title: string; text: string; to: string; action: string }>('GET', `/agenda/booking-check${qs({ slug })}`),
  services: () => request<{ services: ServiceOption[] }>('GET', '/agenda/services').then((r) => r.services),

  blocks: (from?: string, to?: string) => request<{ blocks: TimeBlock[] }>('GET', `/agenda/blocks${qs({ from, to })}`).then((r) => r.blocks),
  createBlock: (input: TimeBlockInput) => request<{ block: TimeBlock; affected: Appt[] }>('POST', '/agenda/blocks', input),
  deleteBlock: (id: string) => request<void>('DELETE', `/agenda/blocks/${seg(id)}`),

  settings: () => request<{ settings: AgendaSettings }>('GET', '/agenda/settings').then((r) => r.settings),
  saveSettings: (s: AgendaSettings) => request<{ settings: AgendaSettings }>('PUT', '/agenda/settings', s).then((r) => r.settings)
};
