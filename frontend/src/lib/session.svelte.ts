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

  has(permission: string): boolean {
    return this.permissions.includes(permission);
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
    if (this.clinic) return;
    try {
      this.clinic = await api.clinic();
    } catch {
      /* the shell works without it */
    }
  }

  async register(r: Parameters<typeof api.register>[0]) {
    this.clinic = null;
    this.user = await api.register(r);
    this.status = 'authenticated';
  }

  async login(email: string, username: string, password: string) {
    this.clinic = null;
    this.user = await api.login(email, username, password);
    this.status = 'authenticated';
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
