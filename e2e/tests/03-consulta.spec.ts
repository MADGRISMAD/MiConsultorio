import { apiAsClinic, createPersonApi, expect, test, ui, uid } from '../support/app';

test('consulta con adenda y receta', async ({ page }) => {
  const api = await apiAsClinic();
  const id = uid();
  const patient = await createPersonApi(api, `Consulta${id}`, 'Paciente');
  await api.dispose();

  await page.goto(`/pacientes/${patient}`);
  await page.getByRole('tab', { name: /Bitácora/ }).click();

  // encounter
  await page.getByRole('button', { name: 'Nueva nota' }).first().click();
  await page.fill(ui.enc.reason, 'Dolor de cabeza');
  await page.fill(ui.enc.subjective, `Refiere dolor desde hace tres días ${id}.`);
  await page.fill(ui.enc.assessment, 'Cefalea tensional');
  await page.getByRole('button', { name: 'Guardar en la bitácora' }).click();
  await expect(page.getByText(`Refiere dolor desde hace tres días ${id}.`)).toBeVisible();

  // addendum: notes cannot be edited or deleted, only clarified
  await page.getByRole('button', { name: 'Agregar adenda' }).first().click();
  await page.fill(ui.addendum.reason, 'Aclaración');
  await page.fill(ui.addendum.text, `El dolor es del lado derecho ${id}.`);
  await page.getByRole('button', { name: 'Guardar adenda' }).click();
  await expect(page.getByText(`El dolor es del lado derecho ${id}.`)).toBeVisible();
  await expect(page.getByRole('button', { name: /Editar nota|Eliminar nota/ })).toHaveCount(0);

  // prescription
  await page.getByRole('tab', { name: /Recetas|Indicaciones/ }).click();
  await page.getByRole('button', { name: 'Nueva receta' }).first().click();
  const dialog = page.locator(ui.dialog);
  await dialog.locator(ui.rx.medication).first().fill('Paracetamol');
  await dialog.locator(ui.rx.dose).first().fill('1 tableta de 500 mg');
  await dialog.locator(ui.rx.frequency).first().fill('Cada 8 horas');
  await dialog.locator(ui.rx.duration).first().fill('3 días');
  await dialog.getByRole('button', { name: 'Crear receta' }).click();
  await expect(page.getByText('Receta lista')).toBeVisible();
  await expect(page.locator(ui.dialog).getByText(/Folio \d+/)).toBeVisible();
  await page.locator(ui.dialog).getByRole('button', { name: 'Cerrar' }).first().click();
  await expect(page.getByRole('tabpanel').getByText(/Folio/).first()).toBeVisible();
});
