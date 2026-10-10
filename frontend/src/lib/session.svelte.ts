import { api, ApiError } from './api';
import type { Clinic, SessionInfo } from './types';

type Status = 'loading' | 'authenticated' | 'anonymous';

// Pista local de "hubo sesión": sin ella, la landing se pinta sin esperar a /api/session.
const HINT = 'caresia:signed';
function hint(on: boolean) {
  try {
    if (on) localStorage.setItem(HINT, '1');
    else localStorage.removeItem(HINT);
  } catch {
    /* sin almacenamiento: solo se pierde el atajo */
  }
}

export function maybeSignedIn(): boolean {
  try {
    return localStorage.getItem(HINT) === '1';
  } catch {
    return true;
  }
}

class SessionStore {
  status = $state<Status>('loading');
  user = $state<SessionInfo | null>(null);
  clinic = $state<Clinic | null>(null);

  get permissions(): string[] {
    return this.user?.permissions ?? [];
  }

  /** Platform staff (the people who run Caresia) live outside any consultorio. */
  get isPlatform(): boolean {
    return this.user?.role === 'platform_admin' || this.user?.role === 'platform_support';
  }

  get isPlatformAdmin(): boolean {
    return this.user?.role === 'platform_admin';
  }

  /** Where a signed-in person lands. */
  get home(): string {
    if (this.isPlatform) return '/plataforma';
    return this.user?.setupPending ? '/bienvenida' : '/';
  }

  has(permission: string): boolean {
    return this.permissions.includes(permission);
  }

  /** The clinic's plan includes cobros (Crecimiento and Pro). */
  get cobros(): boolean {
    return !!this.user?.billing?.cobros;
  }

  /** The plan allows more than one clinic (branches): Pro. */
  get branches(): boolean {
    return (this.user?.billing?.max_branches ?? 1) > 1;
  }

  /** Per-person permissions come with Crecimiento and Pro. */
  get personPermissions(): boolean {
    return !!this.user?.billing?.permissions;
  }

  /** The clinic's plan includes the AI assistant ("magia"): Crecimiento and Pro. */
  get magic(): boolean {
    return (this.user?.billing?.magic_uses ?? 0) > 0;
  }

  /** Subscription blocks clinic data when not usable (platform staff are never blocked). */
  get locked(): boolean {
    return !!this.user?.billing && !this.user.billing.usable;
  }

  async load() {
    try {
      this.user = await api.session();
      this.status = 'authenticated';
      hint(true);
    } catch (e) {
      this.user = null;
      this.status = 'anonymous';
      if (e instanceof ApiError && e.status === 401) hint(false);
      if (!(e instanceof ApiError) || e.status !== 401) console.error(e);
    }
  }

  async loadClinic() {
    if (this.clinic || this.isPlatform) return;
    try {
      this.clinic = await api.clinic();
    } catch {
      /* the shell works without it */
    }
  }

  /** Keep the cached clinic in step with what the server just saved. */
  setClinic(clinic: Clinic) {
    this.clinic = clinic;
  }

  /** Replace the session with a fresh one returned by the server (profile or password changes). */
  setUser(user: SessionInfo) {
    this.user = user;
    this.status = 'authenticated';
    hint(true);
  }

  async register(r: Parameters<typeof api.register>[0]) {
    this.clinic = null;
    this.setUser(await api.register(r));
  }

  async login(identifier: string, password: string) {
    this.clinic = null;
    this.setUser(await api.login(identifier, password));
  }

  async logout() {
    try {
      await api.logout();
    } finally {
      this.user = null;
      this.clinic = null;
      this.status = 'anonymous';
      hint(false);
    }
  }
}

export const session = new SessionStore();
