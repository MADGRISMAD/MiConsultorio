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
    request<{ appointment: Appt }>('POST', `/appointments/${seg(id)}/status`, { status, reason }).then((r) => r.appointment),

  professionals: () => request<{ professionals: Professional[] }>('GET', '/agenda/professionals').then((r) => r.professionals),
  saveProfessional: (id: string, input: ProfessionalInput) =>
    request<{ professional: Professional }>('PUT', `/agenda/professionals/${seg(id)}`, input).then((r) => r.professional),
  services: () => request<{ services: ServiceOption[] }>('GET', '/agenda/services').then((r) => r.services),

  blocks: (from?: string, to?: string) => request<{ blocks: TimeBlock[] }>('GET', `/agenda/blocks${qs({ from, to })}`).then((r) => r.blocks),
  createBlock: (input: TimeBlockInput) => request<{ block: TimeBlock; affected: Appt[] }>('POST', '/agenda/blocks', input),
  deleteBlock: (id: string) => request<void>('DELETE', `/agenda/blocks/${seg(id)}`),

  settings: () => request<{ settings: AgendaSettings }>('GET', '/agenda/settings').then((r) => r.settings),
  saveSettings: (s: AgendaSettings) => request<{ settings: AgendaSettings }>('PUT', '/agenda/settings', s).then((r) => r.settings)
};
