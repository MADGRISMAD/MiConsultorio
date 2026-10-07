// Helpers and selectors shared by the specs. The UI is changing quickly: every selector that depends on
// markup lives in `ui` so it can be adjusted in one place.
import fs from 'node:fs';
import { expect, request as pwRequest, type APIRequestContext, type Page } from '@playwright/test';
import { BASE, CLINIC, MAIL_FILE } from './env.mjs';

export { expect, test } from '@playwright/test';
export { CLINIC, BASE };

export const ui = {
  // sign in / registration
  loginId: '#login-id',
  loginPass: '#login-pass',
  submit: 'button[type=submit]',
  register: { clinic: '#r-name', phone: '#r-phone', person: '#r-person', email: '#r-email', user: '#r-user', pass: '#r-pass', confirm: '#r-confirm' },
  dialog: 'div[role=dialog]',
  // patients
  patient: { names: '#pf-names', lastNames: '#pf-last_names', birth: '#pf-birth_date', phone: '#pf-phone', allergies: '#pf-p-allergies_text', guardianName: '#pf-guardian_name', guardianPhone: '#pf-guardian_phone', ack: '#pf-ack' },
  // encounter and addendum
  enc: { reason: '#enc-reason', subjective: '#enc-subj', assessment: '#enc-ass', plan: '#enc-plan' },
  addendum: { reason: '#add-reason', text: '#add-text' },
  // prescription
  rx: { medication: '[id^="rx-med-"]', dose: '[id^="rx-dose-"]', frequency: '[id^="rx-freq-"]', duration: '[id^="rx-dur-"]' },
  // invoice request
  inv: { rfc: '#inv-rfc', zip: '#inv-zip', name: '#inv-name', regime: '#inv-reg', use: '#inv-use', mail: '#inv-mail' },
  toast: '[role=status]'
};

export async function login(page: Page, username = CLINIC.username, password = CLINIC.password) {
  await page.goto('/login');
  await page.fill(ui.loginId, username);
  await page.fill(ui.loginPass, password);
  await page.click(ui.submit);
  await page.waitForURL((u) => !u.pathname.startsWith('/login'));
}

export async function logout(page: Page) {
  await page.getByRole('button', { name: 'Cerrar sesión' }).first().click();
  await page.locator(ui.dialog).getByRole('button', { name: 'Cerrar sesión' }).click();
  await page.waitForURL((u) => u.pathname === '/login' || u.pathname === '/');
}

/** A date `days` ahead as YYYY-MM-DD, moved forward to a weekday (agenda hours default to Monday to Friday). */
export function futureWeekday(days: number): string {
  const d = new Date();
  d.setDate(d.getDate() + days);
  while (d.getDay() === 0 || d.getDay() === 6) d.setDate(d.getDate() + 1);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

/** Unique-enough suffix so repeated runs against the same database never collide. */
export const uid = () => Date.now().toString(36) + Math.random().toString(36).slice(2, 5);

// ---- API fixtures (signed in as the sample clinic) ----

export async function apiAsClinic(): Promise<APIRequestContext> {
  const api = await pwRequest.newContext({ baseURL: BASE, storageState: '.tmp/state.json' });
  return api;
}

export async function apiJson<T = any>(res: Awaited<ReturnType<APIRequestContext['get']>>): Promise<T> {
  if (!res.ok()) throw new Error(`${res.url()} -> ${res.status()} ${await res.text()}`);
  return res.json();
}

/** Creates a person patient through the API and returns its id. */
export async function createPersonApi(api: APIRequestContext, names: string, lastNames = 'Prueba'): Promise<string> {
  const out = await apiJson(
    await api.post('/api/patients/', {
      data: {
        subject: 'person', names, last_names: lastNames, sex: 'Mujer', birth_date: '1990-05-10', curp: '', phone: '5512345678', email: '', address: '',
        guardian_name: '', guardian_relation: '', guardian_phone: '', guardian_email: '', profile: { allergies_text: 'Ninguna conocida' }, privacy_ack: true
      }
    })
  );
  return out.patient.id;
}

// ---- fake mailbox ----

export const mailbox = {
  clear() {
    fs.writeFileSync(MAIL_FILE, '');
  },
  /** Decoded text of every message received so far (quoted-printable undone, enough to read links). */
  all(): string {
    try {
      return fs.readFileSync(MAIL_FILE, 'utf8').replace(/=\r?\n/g, '').replace(/=3D/g, '=');
    } catch {
      return '';
    }
  }
};

export async function waitForMail(match: RegExp, timeout = 15_000): Promise<string> {
  const end = Date.now() + timeout;
  while (Date.now() < end) {
    const text = mailbox.all();
    const hit = text.match(match);
    if (hit) return hit[0];
    await new Promise((r) => setTimeout(r, 250));
  }
  throw new Error(`no llegó el correo esperado (${match})`);
}
