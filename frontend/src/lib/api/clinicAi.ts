import { request } from '$lib/api';
import type { AiResult } from '$lib/types/specialty';

/** AI helpers of the whole clinic: drafts and suggestions for the owner or the front desk, one magic use each. */
export interface AtRiskPatient {
  id: string;
  name: string;
  last_visit: string;
  days: number;
  reminders_ok: boolean;
}
export type PeriodRange = 'hoy' | 'semana' | 'mes';

export const clinicAiApi = {
  periodSummary: (range: PeriodRange) => request<AiResult>('POST', '/ai/clinic/period-summary', { range }),
  atRisk: () => request<AiResult & { result: AiResult['result'] & { extra: AtRiskPatient[] } }>('POST', '/ai/clinic/at-risk', {}),
  reviewReply: (id: string) => request<AiResult>('POST', '/ai/clinic/review-reply', { id }),
  publicPage: () => request<AiResult>('POST', '/ai/clinic/public-page', {})
};
