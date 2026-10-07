import { request, seg } from '$lib/api';
import type {
  ArcoEvent,
  ArcoNewInput,
  ArcoPackage,
  ArcoPublicInfo,
  ArcoPublicInput,
  ArcoPublicStatus,
  ArcoRequest,
  ArcoSettings,
  ArcoSummary
} from '$lib/types/arco';

export interface ArcoFilters {
  status?: string;
  kind?: string;
  q?: string;
  deadline?: string;
}

const one = (r: { request: ArcoRequest }) => r.request;

export const arcoApi = {
  // Staff (administrador)
  list: (f: ArcoFilters = {}) => {
    const p = new URLSearchParams();
    for (const [k, v] of Object.entries(f)) if (v) p.set(k, v);
    const qs = p.toString();
    return request<{ requests: ArcoRequest[]; summary: ArcoSummary }>('GET', `/arco/${qs ? `?${qs}` : ''}`);
  },
  summary: () => request<{ summary: ArcoSummary }>('GET', '/arco/summary').then((r) => r.summary),
  settings: () => request<ArcoSettings>('GET', '/arco/settings'),
  get: (id: string) => request<{ request: ArcoRequest; events: ArcoEvent[] }>('GET', `/arco/${seg(id)}`),
  create: (body: ArcoNewInput) => request<{ request: ArcoRequest }>('POST', '/arco/', body).then(one),
  patch: (id: string, body: Record<string, unknown>) => request<{ request: ArcoRequest }>('PATCH', `/arco/${seg(id)}`, body).then(one),
  setStatus: (id: string, status: string, note = '', send_email = false) =>
    request<{ request: ArcoRequest }>('POST', `/arco/${seg(id)}/status`, { status, note, send_email }).then(one),
  note: (id: string, note: string) => request<{ request: ArcoRequest }>('POST', `/arco/${seg(id)}/notes`, { note }).then(one),
  respond: (id: string, body: { outcome: 'procedente' | 'improcedente'; response_text: string; denial_reason: string; executed: boolean; send_email: boolean }) =>
    request<{ request: ArcoRequest }>('POST', `/arco/${seg(id)}/respond`, body).then(one),
  execute: (id: string, note = '') => request<{ request: ArcoRequest }>('POST', `/arco/${seg(id)}/execute`, { note }).then(one),
  package: (id: string) => request<{ package: ArcoPackage }>('GET', `/arco/${seg(id)}/package`).then((r) => r.package),
  archivePatient: (id: string) => request<{ request: ArcoRequest }>('POST', `/arco/${seg(id)}/archive-patient`).then(one),
  /** URL de la exportación del expediente (la misma que usa el botón del expediente). */
  exportUrl: (path: string) => `/api${path}`,

  // Public (sin sesión)
  publicInfo: (slug: string) => request<ArcoPublicInfo>('GET', `/public/arco/${seg(slug)}`),
  publicSubmit: (slug: string, body: ArcoPublicInput) =>
    request<{ folio: string; token: string; status_path: string }>('POST', `/public/arco/${seg(slug)}`, body),
  publicStatus: (token: string) => request<{ request: ArcoPublicStatus }>('GET', `/public/arco/status/${seg(token)}`).then((r) => r.request)
};
