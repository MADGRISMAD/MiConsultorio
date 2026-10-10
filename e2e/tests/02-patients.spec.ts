import { expect, test, ui, uid } from '../support/app';

test('registra una persona y un animal', async ({ page }) => {
  const id = uid();

  // person
  await page.goto('/pacientes/nuevo');
  await page.waitForSelector(ui.patient.names);
  await page.getByRole('radio', { name: 'Persona' }).click();
  await page.fill(ui.patient.names, `Persona${id}`);
  await page.fill(ui.patient.lastNames, 'Medina');
  await page.getByRole('radio', { name: 'Hombre' }).click();
  // the date is typed with digits only: the slashes put themselves
  await page.locator(ui.patient.birth).pressSequentially('1203');
  await page.getByRole('button', { name: 'Registrar paciente' }).click();
  await expect(page.getByText('Escribe la fecha completa')).toBeVisible(); // incomplete: not silently dropped
  await page.locator(ui.patient.birth).pressSequentially('1980');
  await expect(page.locator(ui.patient.birth)).toHaveValue('12/03/1980');
  await page.fill(ui.patient.phone, '5512345678');
  await page.fill(ui.patient.allergies, 'Ninguna conocida');
  await page.check(ui.patient.ack);
  await page.getByRole('button', { name: 'Registrar paciente' }).click();
  await page.waitForURL(/\/pacientes\/[0-9a-f-]{36}$/);
  await expect(page.getByText(`Persona${id}`).first()).toBeVisible();
  await expect(page.getByText(/4\d años/).first()).toBeVisible(); // the typed date was saved

  // animal: the owner is required
  await page.goto('/pacientes/nuevo');
  await page.waitForSelector(ui.patient.names);
  await page.getByRole('radio', { name: 'Animal' }).click();
  await page.fill(ui.patient.names, `Firulais${id}`);
  await page.getByRole('radio', { name: 'Macho' }).click();
  await page.locator('#pf-p-species').selectOption('Perro');
  await page.locator(ui.patient.ownerNames).fill('Ana');
  await page.locator(ui.patient.ownerSurnames).fill('Propietaria');
  await page.locator(ui.patient.guardianPhone).fill('5598765432');
  await page.fill(ui.patient.allergies, 'Ninguna conocida');
  await page.check(ui.patient.ack);
  await page.getByRole('button', { name: 'Registrar paciente' }).click();
  await page.waitForURL(/\/pacientes\/[0-9a-f-]{36}$/);
  await expect(page.getByText(`Firulais${id}`).first()).toBeVisible();

  // both appear in the list
  await page.goto('/pacientes');
  await expect(page.getByText(`Medina`).first()).toBeVisible();
  await expect(page.getByText(`Firulais${id}`).first()).toBeVisible();
});
