import { request, seg } from '$lib/api';
import type {
  PortalAppointment,
  PortalInfo,
  PortalMe,
  PortalPrescription,
  PortalSettings,
  PortalVaccination
} from '$lib/types/portal';

/** Public portal endpoints. The session is its own cookie, unrelated to the staff session. */
export const portalApi = {
  info: (slug: string) => request<{ clinic: PortalInfo }>('GET', `/portal/${seg(slug)}/info`).then((r) => r.clinic),
  requestCode: (slug: string, email: string) => request<{ ok: boolean }>('POST', `/portal/${seg(slug)}/code`, { email }),
  login: (slug: string, email: string, code: string) => request<{ ok: boolean }>('POST', `/portal/${seg(slug)}/login`, { email, code }),
  logout: () => request<void>('POST', '/portal/logout'),
  me: () => request<PortalMe>('GET', '/portal/me'),
  appointments: () =>
    request<{ upcoming: PortalAppointment[]; history: PortalAppointment[]; cancel_min_hours: number }>('GET', '/portal/appointments'),
  cancel: (id: string, reason: string) => request<{ ok: boolean }>('POST', `/portal/appointments/${seg(id)}/cancel`, { reason }),
  prescriptions: () =>
    request<{ prescriptions: PortalPrescription[]; by_area?: boolean; clinic: { name: string; address: string; phone: string } }>('GET', '/portal/prescriptions'),
  vaccinations: () => request<{ vaccinations: PortalVaccination[] }>('GET', '/portal/vaccinations'),

  // clinic side (staff session)
  settings: () => request<{ portal: PortalSettings }>('GET', '/clinic/portal').then((r) => r.portal),
  saveSettings: (body: { enabled: boolean; welcome: string }) =>
    request<{ portal: PortalSettings }>('PUT', '/clinic/portal', body).then((r) => r.portal)
};
