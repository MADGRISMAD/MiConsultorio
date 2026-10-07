import { expect, futureWeekday, test, ui, uid } from '../support/app';

// Two appointments for the same professional at the same time: the second one is refused until it is booked as an overbooking.
test('una cita que choca con otra avisa del conflicto', async ({ page }) => {
  const id = uid();
  const date = futureWeekday(7);
  await page.goto('/admin/admin-citas');
  await expect(page.getByRole('heading', { name: 'Administrar citas' })).toBeVisible();

  const book = async (names: string) => {
    await page.getByRole('button', { name: 'Nueva cita' }).first().click();
    const dialog = page.locator(ui.dialog);
    await dialog.getByLabel('Nombre(s)').fill(names);
    await dialog.getByLabel('Apellido(s)').fill('Agenda');
    await dialog.getByLabel('Fecha').fill(date);
    await dialog.getByLabel('Profesional').selectOption({ index: 1 }); // the same professional both times
    await dialog.getByLabel('Hora de inicio').fill('10:00');
    await dialog.getByLabel('Hora de finalización').fill('10:30');
    await dialog.getByRole('button', { name: /^Guardar/ }).click();
    return dialog;
  };

  const first = await book(`Primera${id}`);
  await expect(first).toBeHidden();

  const second = await book(`Segunda${id}`);
  await expect(second.getByRole('alert')).toContainText(/ocupad|ya tiene|encima|horario/i);
  await expect(second.getByText('Agendar como sobreturno')).toBeVisible();

  // cancelling the form leaves only the first appointment
  await second.getByRole('button', { name: 'Cancelar' }).click();
  await expect(second).toBeHidden();
});
