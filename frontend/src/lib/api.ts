import type {
  ActivityItem,
  Appointment,
  AppointmentInput,
  Clinic,
  ClinicRow,
  ClinicSettings,
  Expedient,
  ExpedientInput,
  Overview,
  Payment,
  Person,
  Plan,
  Seats,
  SessionInfo
} from './types';

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number
  ) {
    super(message);
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  let res: Response;
  try {
    res = await fetch(`/api${path}`, {
      method,
      credentials: 'same-origin',
      headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body)
    });
  } catch {
    throw new ApiError('No se pudo conectar con el servidor.', 0);
  }
  if (res.status === 204) return undefined as T;
  const data = await res.json().catch(() => null);
  if (!res.ok) throw new ApiError(data?.message ?? 'Ocurrió un error inesperado.', res.status);
  return data as T;
}

const seg = encodeURIComponent;

export const api = {
  login: (identifier: string, password: string) =>
    request<{ session: SessionInfo }>('POST', '/login', { identifier, password }).then((r) => r.session),
  register: (r: { clinic_name: string; phone: string; name: string; email: string; username: string; password: string }) =>
    request<{ session: SessionInfo }>('POST', '/register', r).then((x) => x.session),
  logout: () => request<void>('POST', '/logout'),
  session: () => request<{ session: SessionInfo }>('GET', '/session').then((r) => r.session),
  clinic: () => request<{ clinic: Clinic }>('GET', '/clinic').then((r) => r.clinic),
  updateClinic: (patch: Partial<{ name: string; phone_number: string; address: string; kind: string; specialties: string[]; settings: ClinicSettings }>) =>
    request<{ clinic: Clinic }>('PUT', '/clinic', patch).then((r) => r.clinic),
  completeSetup: () => request<{ session: SessionInfo }>('POST', '/clinic/setup').then((r) => r.session),

  // my account
  updateProfile: (name: string, phone: string) => request<{ session: SessionInfo }>('PUT', '/me', { name, phone }).then((r) => r.session),
  changePassword: (current_password: string, new_password: string) =>
    request<{ session: SessionInfo }>('PUT', '/me/password', { current_password, new_password }).then((r) => r.session),

  // clinic team
  team: () => request<{ people: Person[]; seats: Seats; roles: { id: string; label: string }[] }>('GET', '/team/'),
  addMember: (m: { name: string; email: string; username: string; phone: string; password: string; role: string }) =>
    request<{ person: Person }>('POST', '/team/', m),
  updateMember: (id: string, patch: Partial<{ name: string; email: string; phone: string; role: string }>) =>
    request<{ person: Person }>('PATCH', `/team/${seg(id)}`, patch),
  resetMemberPassword: (id: string, password: string) => request<void>('POST', `/team/${seg(id)}/password`, { password }),
  deactivateMember: (id: string) => request<void>('POST', `/team/${seg(id)}/deactivate`),
  reactivateMember: (id: string) => request<void>('POST', `/team/${seg(id)}/reactivate`),

  expedients: () => request<{ expedients: Expedient[] }>('GET', '/expedients/').then((r) => r.expedients),
  expedient: (curp: string) => request<{ expedient: Expedient }>('GET', `/expedients/${seg(curp)}`).then((r) => r.expedient),
  createExpedient: (e: ExpedientInput) => request<unknown>('POST', '/expedients/', e),
  updateExpedient: (curp: string, e: ExpedientInput) => request<unknown>('PUT', `/expedients/${seg(curp)}`, e),
  deleteExpedient: (curp: string) => request<void>('DELETE', `/expedients/${seg(curp)}`),

  appointments: () => request<{ appointments: Appointment[] }>('GET', '/appointments/').then((r) => r.appointments),
  appointment: (id: string) => request<{ appointment: Appointment }>('GET', `/appointments/${seg(id)}`).then((r) => r.appointment),
  createAppointment: (a: AppointmentInput) => request<unknown>('POST', '/appointments/', a),
  updateAppointment: (id: string, a: AppointmentInput) => request<unknown>('PUT', `/appointments/${seg(id)}`, a),
  deleteAppointment: (id: string) => request<void>('DELETE', `/appointments/${seg(id)}`),

  // platform
  platform: {
    overview: () => request<Overview>('GET', '/platform/overview'),
    plans: () => request<{ plans: Plan[] }>('GET', '/platform/plans').then((r) => r.plans),
    clinics: (q = '', state = '') =>
      request<{ clinics: ClinicRow[]; counts: Record<string, number> }>('GET', `/platform/clinics?q=${encodeURIComponent(q)}&state=${encodeURIComponent(state)}`),
    clinic: (id: string) =>
      request<{ clinic: ClinicRow; people: Person[]; seats: Seats; activity: ActivityItem[]; payments: Payment[] }>('GET', `/platform/clinics/${seg(id)}`),
    updateClinic: (id: string, patch: Record<string, unknown>) => request<{ clinic: ClinicRow }>('PATCH', `/platform/clinics/${seg(id)}`, patch).then((r) => r.clinic),
    suspend: (id: string, reason: string) => request<{ clinic: ClinicRow }>('POST', `/platform/clinics/${seg(id)}/suspend`, { reason }).then((r) => r.clinic),
    reactivate: (id: string) => request<{ clinic: ClinicRow }>('POST', `/platform/clinics/${seg(id)}/reactivate`).then((r) => r.clinic),
    pay: (id: string, p: { amount: number; months: number; note: string }) => request<{ period_end: string }>('POST', `/platform/clinics/${seg(id)}/payments`, p),
    activity: (clinicId = '') => request<{ activity: ActivityItem[] }>('GET', `/platform/activity?limit=200${clinicId ? `&clinic_id=${seg(clinicId)}` : ''}`).then((r) => r.activity),
    staff: () => request<{ people: Person[]; roles: { id: string; label: string }[] }>('GET', '/platform/staff'),
    addStaff: (m: { name: string; email: string; username: string; password: string; role: string }) => request<{ person: Person }>('POST', '/platform/staff', m),
    updateStaff: (id: string, patch: Partial<{ name: string; role: string }>) => request<{ person: Person }>('PATCH', `/platform/staff/${seg(id)}`, patch),
    resetStaffPassword: (id: string, password: string) => request<void>('POST', `/platform/staff/${seg(id)}/password`, { password }),
    deactivateStaff: (id: string) => request<void>('POST', `/platform/staff/${seg(id)}/deactivate`),
    reactivateStaff: (id: string) => request<void>('POST', `/platform/staff/${seg(id)}/reactivate`)
  }
};
