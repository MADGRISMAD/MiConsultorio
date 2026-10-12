import { request, seg } from '$lib/api';

export interface SessionPackage {
  id: string;
  name: string;
  price_cents: number;
  sessions: number;
  service_id: string | null;
  service_name: string;
  valid_days: number | null;
  public: boolean;
  active: boolean;
}

export interface SessionPackageInput {
  name: string;
  price_cents: number;
  sessions: number;
  service_id: string | null;
  valid_days: number | null;
  public: boolean;
  active: boolean;
}

/** A package sold to a patient, with its balance. */
export interface PatientPackage {
  id: string;
  name: string;
  service_id: string | null;
  service_name: string;
  sessions_total: number;
  sessions_used: number;
  expires_on: string | null;
  expired: boolean;
  status: 'active' | 'void';
  sold_at: string;
}

export const packagesApi = {
  list: () => request<{ packages: SessionPackage[]; enabled: boolean }>('GET', '/packages'),
  save: (id: string | null, input: SessionPackageInput) =>
    request<{ package: SessionPackage }>(id ? 'PUT' : 'POST', id ? `/packages/${seg(id)}` : '/packages', input).then((r) => r.package),
  ofPatient: (patientId: string) => request<{ packages: PatientPackage[] }>('GET', `/patients/${seg(patientId)}/packages`).then((r) => r.packages),
  redeem: (apptId: string, packageId: string) =>
    request<{ sessions_left: number }>('POST', `/appointments/${seg(apptId)}/package`, { package_id: packageId }).then((r) => r.sessions_left),
  unredeem: (apptId: string) => request<void>('DELETE', `/appointments/${seg(apptId)}/package`)
};

/** Sessions still available in a package of a patient on a day (YYYY-MM-DD), for the given service. */
export function usablePackages(list: PatientPackage[], serviceId: string | null | undefined, day: string): PatientPackage[] {
  return list.filter(
    (p) =>
      p.status === 'active' &&
      p.sessions_used < p.sessions_total &&
      (!p.expires_on || p.expires_on >= day) &&
      (!p.service_id || p.service_id === serviceId)
  );
}
