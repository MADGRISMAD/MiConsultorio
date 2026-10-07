import { request, seg } from '$lib/api';
import type { SessionInfo } from '$lib/types';
import type { LoginResult, SecurityMember, TwoFactorPolicy, TwoFactorSetup } from '$lib/types/security';

export const securityApi = {
  /** Como api.login, pero entiende que la cuenta pida un segundo paso. */
  async login(identifier: string, password: string): Promise<LoginResult> {
    const r = await request<{ session?: SessionInfo; needs_2fa?: boolean; challenge?: string }>('POST', '/login', { identifier, password });
    if (r.needs_2fa && r.challenge) return { needs_2fa: true, challenge: r.challenge };
    return { needs_2fa: false, session: r.session as SessionInfo };
  },
  /** Segundo paso: un código de la app o un código de recuperación. */
  login2fa: (challenge: string, code: string, recovery: boolean) =>
    request<{ session: SessionInfo; recovery_codes_left?: number }>('POST', '/login/2fa', recovery ? { challenge, recovery_code: code } : { challenge, code }),

  setup: () => request<TwoFactorSetup>('POST', '/me/2fa/setup'),
  enable: (code: string) => request<{ session: SessionInfo; recovery_codes: string[] }>('POST', '/me/2fa/enable', { code }),
  disable: (password: string, code: string) => request<{ session: SessionInfo }>('POST', '/me/2fa/disable', { password, code }),
  newRecoveryCodes: (password: string, code: string) =>
    request<{ session: SessionInfo; recovery_codes: string[] }>('POST', '/me/2fa/recovery-codes', { password, code }),

  policy: () => request<{ require_2fa: TwoFactorPolicy }>('GET', '/security/policy').then((r) => r.require_2fa),
  setPolicy: (require_2fa: TwoFactorPolicy) => request<{ require_2fa: TwoFactorPolicy }>('PUT', '/security/policy', { require_2fa }),
  members: () => request<{ members: SecurityMember[] }>('GET', '/security/members').then((r) => r.members),
  resetMember: (id: string) => request<{ ok: boolean }>('POST', `/security/members/${seg(id)}/2fa/reset`),

  /** URL de la exportación del expediente (JSON) para ARCO y portabilidad. */
  exportUrl: (patientId: string) => `/api/patients/${seg(patientId)}/export`
};
