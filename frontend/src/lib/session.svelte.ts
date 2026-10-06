import { api, ApiError } from './api';
import type { Clinic, SessionInfo } from './types';

type Status = 'loading' | 'authenticated' | 'anonymous';

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

  /** Subscription blocks clinic data when not usable (platform staff are never blocked). */
  get locked(): boolean {
    return !!this.user?.billing && !this.user.billing.usable;
  }

  async load() {
    try {
      this.user = await api.session();
      this.status = 'authenticated';
    } catch (e) {
      this.user = null;
      this.status = 'anonymous';
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
    }
  }
}

export const session = new SessionStore();
