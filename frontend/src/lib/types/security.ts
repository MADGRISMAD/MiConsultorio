import type { Role, SessionInfo } from '$lib/types';

export type TwoFactorPolicy = 'none' | 'admins' | 'clinical' | 'all';

export interface TwoFactorSetup {
  /** base32, para escribirlo a mano si no se puede escanear el QR */
  secret: string;
  otpauth_uri: string;
  issuer: string;
  account: string;
}

export interface SecurityMember {
  id: string;
  name: string;
  username: string;
  role: Role;
  role_label: string;
  disabled: boolean;
  two_factor_enabled: boolean;
}

/** Respuesta de POST /login: o ya hay sesión, o falta el segundo paso. */
export type LoginResult = { needs_2fa: false; session: SessionInfo } | { needs_2fa: true; challenge: string };
