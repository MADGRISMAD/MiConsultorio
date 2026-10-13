import { apiAsClinic, createPersonApi, createStartedApptApi, expect, test, ui, uid } from '../support/app';

// Va al final de la suite a propósito: al completar la cita deja una pre-cuenta en caja, y las pruebas de cobros esperan caja limpia.
test('desde el inicio: iniciar la consulta de una cita abre la nota ligada y al guardarla la cita se completa', async ({ page }) => {
  const id = uid();
  const names = `Inicio${id}`;
  const api = await apiAsClinic();
  const patient = await createPersonApi(api, names, 'Consulta');
  await createStartedApptApi(api, { patientId: patient, names, lastNames: 'Consulta', details: 'Revisión general' });
  await api.dispose();

  await page.goto('/');
  const row = page.getByRole('listitem').filter({ hasText: names });
  await expect(row).toBeVisible();
  await expect(row.getByText('Por confirmar')).toBeVisible();

  // un clic: marca llegada, pasa a consulta y abre el expediente con la nota lista
  await row.getByRole('button', { name: 'Iniciar consulta' }).click();
  await page.waitForURL(new RegExp(`/pacientes/${patient}`));
  await expect(page.locator(ui.enc.reason)).toHaveValue('Revisión general');
  await page.fill(ui.enc.subjective, `Viene a revisión ${id}.`);
  await page.fill(ui.enc.assessment, 'Sin hallazgos');
  await page.getByRole('button', { name: 'Guardar en la bitácora' }).click();
  await expect(page.getByText(`Viene a revisión ${id}.`)).toBeVisible();

  // la cita quedó atendida
  await page.goto('/');
  await expect(page.getByRole('listitem').filter({ hasText: names }).getByText('Completada')).toBeVisible();
});
