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
  await page.fill(ui.patient.birth, '1980-03-12');
  await page.fill(ui.patient.phone, '5512345678');
  await page.fill(ui.patient.allergies, 'Ninguna conocida');
  await page.check(ui.patient.ack);
  await page.getByRole('button', { name: 'Registrar paciente' }).click();
  await page.waitForURL(/\/pacientes\/[0-9a-f-]{36}$/);
  await expect(page.getByText(`Persona${id}`).first()).toBeVisible();

  // animal: the owner is required
  await page.goto('/pacientes/nuevo');
  await page.waitForSelector(ui.patient.names);
  await page.getByRole('radio', { name: 'Animal' }).click();
  await page.fill(ui.patient.names, `Firulais${id}`);
  await page.getByRole('radio', { name: 'Macho' }).click();
  await page.locator('#pf-p-species').selectOption('Perro');
  await page.locator(ui.patient.guardianName).fill('Ana Propietaria');
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
