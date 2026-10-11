import { apiAsClinic, apiJson, expect, test, ui, uid } from '../support/app';

// Misma zona que el consultorio de ejemplo para que «hoy» coincida entre navegador y servidor.
const TZ = 'America/Mexico_City';
test.use({ timezoneId: TZ });

test('registrar al paciente de una cita: el formulario ya trae sus datos y la cita queda ligada', async ({ page }) => {
  const id = uid();
  const names = `Cita${id}`;
  const api = await apiAsClinic();
  const pros = (await apiJson(await api.get('/api/agenda/professionals'))).professionals as { id: string; consults: boolean }[];
  const p = Object.fromEntries(
    new Intl.DateTimeFormat('en-CA', { timeZone: TZ, year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hourCycle: 'h23' })
      .formatToParts(new Date())
      .map((x) => [x.type, x.value])
  );
  const start = Math.min(Math.max(0, Number(p.hour) * 60 + Number(p.minute) - 30), 23 * 60);
  const hhmm = (m: number) => `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`;
  await apiJson(
    await api.post('/api/appointments/', {
      data: {
        patient_id: null, names, last_names: 'Reservó', CURP: '', date: `${p.year}-${p.month}-${p.day}`, startHour: hhmm(start), endHour: hhmm(start + 20), details: 'Primera vez',
        professional_id: (pros.find((x) => x.consults) ?? pros[0]).id, service_id: null, room: '', phone: '5512340099', email: `${names.toLowerCase()}@ejemplo.test`, reminders_consent: false, overbook: true
      }
    })
  );
  await api.dispose();

  // el inicio avisa que aún no es paciente y ofrece registrarlo
  await page.goto('/');
  const row = page.getByRole('listitem').filter({ hasText: names });
  await expect(row).toBeVisible();
  await row.getByRole('button', { name: 'Registrar paciente' }).click();

  // el formulario viene con lo que ya se sabe de la cita
  await page.waitForURL(/\/pacientes\/nuevo\?cita=/);
  await expect(page.getByText(/Datos tomados de la cita/)).toBeVisible();
  await expect(page.locator(ui.patient.names)).toHaveValue(names);
  await expect(page.locator(ui.patient.lastNames)).toHaveValue('Reservó');
  await expect(page.locator(ui.patient.phone)).toHaveValue('5512340099');
  await expect(page.locator('#pf-email')).toHaveValue(`${names.toLowerCase()}@ejemplo.test`);

  // solo falta lo que la cita no podía saber
  await page.getByRole('radio', { name: 'Mujer' }).click();
  await page.locator(ui.patient.birth).pressSequentially('05101990');
  await page.fill(ui.patient.allergies, 'Ninguna conocida');
  await page.check(ui.patient.ack);
  await page.getByRole('button', { name: 'Registrar paciente' }).click();

  // vuelve al inicio y la cita ya tiene paciente: su siguiente paso es iniciar la consulta
  await page.waitForURL((u) => u.pathname === '/');
  await expect(page.getByRole('listitem').filter({ hasText: names }).getByRole('button', { name: 'Iniciar consulta' })).toBeVisible();
  await expect(page.getByRole('listitem').filter({ hasText: names }).getByRole('button', { name: 'Registrar paciente' })).toHaveCount(0);
});
