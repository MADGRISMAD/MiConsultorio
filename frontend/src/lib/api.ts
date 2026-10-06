import type {
  Appointment,
  AppointmentInput,
  Clinic,
  Expedient,
  ExpedientInput,
  SessionInfo,
  User
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
  login: (email: string, username: string, password: string) =>
    request<{ session: SessionInfo }>('POST', '/login', { email, username, password }).then((r) => r.session),
  logout: () => request<void>('POST', '/logout'),
  session: () => request<{ session: SessionInfo }>('GET', '/session').then((r) => r.session),
  clinic: () => request<{ clinic: Clinic }>('GET', '/clinic').then((r) => r.clinic),

  users: () => request<{ users: User[] }>('GET', '/users/').then((r) => r.users),
  createUser: (u: { username: string; password: string; permissions: string[] }) => request<unknown>('POST', '/users/', u),
  updateUserPassword: (username: string, password: string) => request<void>('PUT', `/users/${seg(username)}`, { password }),
  updatePermissions: (users: Record<string, string[]>) => request<void>('PUT', '/users/permissions', { users }),
  deleteUser: (username: string) => request<void>('DELETE', `/users/${seg(username)}`),

  expedients: () => request<{ expedients: Expedient[] }>('GET', '/expedients/').then((r) => r.expedients),
  expedient: (curp: string) => request<{ expedient: Expedient }>('GET', `/expedients/${seg(curp)}`).then((r) => r.expedient),
  createExpedient: (e: ExpedientInput) => request<unknown>('POST', '/expedients/', e),
  updateExpedient: (curp: string, e: ExpedientInput) => request<unknown>('PUT', `/expedients/${seg(curp)}`, e),
  deleteExpedient: (curp: string) => request<void>('DELETE', `/expedients/${seg(curp)}`),

  appointments: () => request<{ appointments: Appointment[] }>('GET', '/appointments/').then((r) => r.appointments),
  appointment: (id: string) => request<{ appointment: Appointment }>('GET', `/appointments/${seg(id)}`).then((r) => r.appointment),
  createAppointment: (a: AppointmentInput) => request<unknown>('POST', '/appointments/', a),
  updateAppointment: (id: string, a: AppointmentInput) => request<unknown>('PUT', `/appointments/${seg(id)}`, a),
  deleteAppointment: (id: string) => request<void>('DELETE', `/appointments/${seg(id)}`)
};
