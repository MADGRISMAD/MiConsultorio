import { request, seg } from '$lib/api';
import type { OrgBranch, OrgBranchInput, OrgOverview, OrgSummary } from '$lib/types/org';

export const orgApi = {
  overview: () => request<OrgOverview>('GET', '/org'),
  rename: (name: string) => request<{ organization: { id: string; name: string } }>('PUT', '/org', { name }),
  createBranch: (b: OrgBranchInput) => request<{ branch: OrgBranch }>('POST', '/org/branches', b).then((r) => r.branch),
  suspend: (id: string) => request<{ branches: OrgBranch[] }>('POST', `/org/branches/${seg(id)}/suspend`).then((r) => r.branches),
  reactivate: (id: string) => request<{ branches: OrgBranch[] }>('POST', `/org/branches/${seg(id)}/reactivate`).then((r) => r.branches),
  /** Signs a new session in the chosen branch. The caller reloads the app so nothing from the old clinic stays on screen. */
  switchTo: (branchId: string) => request<unknown>('POST', '/org/switch', { branch_id: branchId }),
  summary: (from: string, to: string) => request<OrgSummary>('GET', `/org/reports/summary?from=${seg(from)}&to=${seg(to)}`),
  csvUrl: (from: string, to: string) => `/api/org/reports/summary.csv?from=${seg(from)}&to=${seg(to)}`
};

/** Enters a branch and reloads the whole app on its home page. */
export async function enterBranch(branchId: string) {
  await orgApi.switchTo(branchId);
  window.location.assign('/');
}
