import { api, ApiError } from './api';
import type { SessionInfo } from './types';

type Status = 'loading' | 'authenticated' | 'anonymous';

class SessionStore {
  status = $state<Status>('loading');
  user = $state<SessionInfo | null>(null);

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

  async login(email: string, username: string, password: string) {
    this.user = await api.login(email, username, password);
    this.status = 'authenticated';
  }

  async logout() {
    try {
      await api.logout();
    } finally {
      this.user = null;
      this.status = 'anonymous';
    }
  }
}

export const session = new SessionStore();
