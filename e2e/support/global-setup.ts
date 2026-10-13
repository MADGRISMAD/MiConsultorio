import fs from 'node:fs';
import { request } from '@playwright/test';
import { BASE, CLINIC } from './env.mjs';

// Runs once after the server is up: signs the sample clinic in (saving the session for every test) and
// prepares it the way a real clinic would be (people and animals, cédula, legal data) so the specs can focus on flows.
export default async function globalSetup() {
  const api = await request.newContext({ baseURL: BASE });
  const check = async (res: Awaited<ReturnType<typeof api.get>>, what: string) => {
    if (!res.ok()) throw new Error(`global setup: ${what} -> ${res.status()} ${await res.text()}`);
  };
  await check(await api.post('/api/login', { data: { identifier: CLINIC.username, password: CLINIC.password } }), 'login');
  await check(await api.put('/api/clinic', { data: { specialties: ['VETERINARY'] } }), 'clinic specialties');
  await check(
    await api.put('/api/me', {
      data: { name: CLINIC.name, phone: '', cedula: '12345678', cedula_institution: 'UNAM', cedula_specialty: '', specialty_title: 'Médico Cirujano' }
    }),
    'professional data'
  );
  await check(
    await api.put('/api/clinic/legal', {
      data: {
        responsible_name: 'Dr. Responsable',
        responsible_license: '7654321',
        responsible_institution: 'UNAM',
        operating_notice: 'AF-2026-1',
        privacy_contact: 'Administración',
        privacy_email: 'privacidad@clinica.test',
        privacy_phone: '',
        privacy_address: ''
      }
    }),
    'legal data'
  );
  // the consent banner of the public pages must not sit over a button during a test: the state carries a choice already made
  const state = await api.storageState();
  state.origins = [{ origin: BASE, localStorage: [{ name: 'caresia-cookies', value: 'rechazadas' }] }];
  fs.mkdirSync('.tmp', { recursive: true });
  fs.writeFileSync('.tmp/state.json', JSON.stringify(state));
  await api.dispose();
}
