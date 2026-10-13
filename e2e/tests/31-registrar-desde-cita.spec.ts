import { apiAsClinic, createStartedApptApi, expect, test, ui, uid } from '../support/app';

test('registrar al paciente de una cita: el formulario ya trae sus datos y la cita queda ligada', async ({ page }) => {
  const id = uid();
  const names = `Cita${id}`;
  const api = await apiAsClinic();
  await createStartedApptApi(api, { patientId: null, names, lastNames: 'Reservó', details: 'Primera vez', phone: '5512340099', email: `${names.toLowerCase()}@ejemplo.test` });
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
