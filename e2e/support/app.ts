// Helpers and selectors shared by the specs. The UI is changing quickly: every selector that depends on
// markup lives in `ui` so it can be adjusted in one place.
import fs from 'node:fs';
import { expect, request as pwRequest, type APIRequestContext, type Page } from '@playwright/test';
import { BASE, CLINIC, MAIL_FILE, PLATFORM } from './env.mjs';

export { expect, test } from '@playwright/test';
export { CLINIC, BASE, PLATFORM };

export const ui = {
  // sign in / registration
  loginId: '#login-id',
  loginPass: '#login-pass',
  submit: 'button[type=submit]',
  register: { clinic: '#r-name', phone: '#r-phone', person: '#r-person', email: '#r-email', user: '#r-user', pass: '#r-pass', confirm: '#r-confirm' },
  dialog: 'div[role=dialog]',
  // patients
  patient: { names: '#pf-names', lastNames: '#pf-last_names', birth: '#pf-birth_date', phone: '#pf-phone', allergies: '#pf-p-allergies_text', guardianName: '#pf-guardian_name', ownerNames: '#pf-owner_names', ownerSurnames: '#pf-owner_surnames', guardianPhone: '#pf-guardian_phone', ack: '#pf-ack' },
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

/** An API context with no session (to register a clinic of its own). */
export const apiAnon = () => pwRequest.newContext({ baseURL: BASE });

/** An API session signed in with the given account (the platform administrator, a clinic registered by the test...). */
export async function apiLogin(identifier: string, password: string): Promise<APIRequestContext> {
  const api = await apiAnon();
  await apiJson(await api.post('/api/login', { data: { identifier, password } }));
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

/**
 * Gives the clinic one more specialty for the length of `run` (the plan allows three giros in all, so a test adds one) and puts
 * the previous list back afterwards, checking that the restore worked. `run` receives a new person patient and the API context.
 */
export async function withSpecialty(kind: string, run: (patientId: string, api: APIRequestContext) => Promise<void>): Promise<void> {
  const api = await apiAsClinic();
  const before = (await apiJson(await api.get('/api/clinic'))).clinic;
  await apiJson(await api.put('/api/clinic', { data: { specialties: [kind] } }));
  try {
    await run(await createPersonApi(api, `Esp${uid()}`), api);
  } finally {
    const res = await api.put('/api/clinic', { data: { specialties: before.specialties ?? [] } });
    await api.dispose();
    expect(res.ok(), 'the clinic must get its specialties back').toBe(true);
  }
}

/** Fecha y hora actuales en la zona del consultorio de ejemplo (la misma que usa el navegador de las pruebas). */
export function clinicNow() {
  const p = Object.fromEntries(
    new Intl.DateTimeFormat('en-CA', { timeZone: 'America/Mexico_City', year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hourCycle: 'h23' })
      .formatToParts(new Date())
      .map((x) => [x.type, x.value])
  );
  return { date: `${p.year}-${p.month}-${p.day}`, minutes: Number(p.hour) * 60 + Number(p.minute) };
}

export const hhmm = (m: number) => `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`;

/** Una cita de hoy que ya es «su hora» a cualquier hora del día (para poder iniciar la consulta). */
export async function createStartedApptApi(
  api: APIRequestContext,
  a: { patientId: string | null; names: string; lastNames: string; details: string; phone?: string; email?: string }
): Promise<void> {
  const pros = (await apiJson(await api.get('/api/agenda/professionals'))).professionals as { id: string; consults: boolean }[];
  const { date, minutes } = clinicNow();
  const start = Math.min(Math.max(0, minutes - 30), 23 * 60);
  await apiJson(
    await api.post('/api/appointments/', {
      data: {
        patient_id: a.patientId, names: a.names, last_names: a.lastNames, CURP: '', date, startHour: hhmm(start), endHour: hhmm(start + 20), details: a.details,
        professional_id: (pros.find((p) => p.consults) ?? pros[0]).id, service_id: null, room: '', phone: a.phone ?? '', email: a.email ?? '', reminders_consent: false, overbook: true
      }
    })
  );
}

/** A browser with no session, but with the cookie choice already made (the consent banner would cover buttons otherwise). */
export const ANON_STATE = { cookies: [], origins: [{ origin: BASE, localStorage: [{ name: 'caresia-cookies', value: 'rechazadas' }] }] };
