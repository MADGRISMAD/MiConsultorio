import { request, seg } from '$lib/api';
import type { PatientRow } from '$lib/types';
import type { OwnerGroup, OwnerListItem, OwnerPet, OwnerRef } from '$lib/types/owners';

export const ownersApi = {
  /** owners with their pets, for the pickers of the forms */
  search: (q: string) => request<{ owners: OwnerListItem[] }>('GET', `/patients/owners?q=${encodeURIComponent(q)}`).then((r) => r.owners),
  get: (id: string) => request<{ owner: OwnerListItem }>('GET', `/patients/owners/${seg(id)}`).then((r) => r.owner),
  /** the animals of the clinic grouped by owner (and the ones without an owner registered) */
  grouped: (p: { q?: string; archived?: boolean } = {}) => {
    const q = new URLSearchParams();
    if (p.q) q.set('q', p.q);
    if (p.archived) q.set('archived', '1');
    return request<{ groups: OwnerGroup[]; orphans: PatientRow[] }>('GET', `/patients/grouped?${q}`);
  },
  /** the owner of a pet and its siblings */
  ofPatient: (patientId: string) => request<{ owner: OwnerRef | null; siblings: OwnerPet[] }>('GET', `/patients/${seg(patientId)}/owner`),
  update: (id: string, o: { name: string; phone: string; email: string }) => request<{ owner: OwnerRef }>('PUT', `/patients/owners/${seg(id)}`, o).then((r) => r.owner),
  merge: (id: string, into: string) => request<{ moved: number }>('POST', `/patients/owners/${seg(id)}/merge`, { into })
};
