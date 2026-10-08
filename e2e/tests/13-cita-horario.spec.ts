import { apiAsClinic, apiJson, expect, futureWeekday, test, ui, uid } from '../support/app';

// A visit cannot be received, started or closed before its time (and the API refuses it too).
test('una cita de otro día no se puede marcar como llegada ni terminar', async ({ page }) => {
  const api = await apiAsClinic();
  const id = uid();
  const date = futureWeekday(10);
  const made = await apiJson(
    await api.post('/api/appointments', { data: { names: `Anticipada${id}`, last_names: 'Horario', date, startHour: '09:00', endHour: '09:30', details: '' } })
  );
  const apptId = made.appointment.id as string;

  // the server says no, with its reason
  const early = await api.post(`/api/appointments/${apptId}/status`, { data: { status: 'arrived' } });
  expect(early.status()).toBe(409);
  expect((await early.json()).code).toBe('TOO_EARLY');
  expect((await api.post(`/api/appointments/${apptId}/status`, { data: { status: 'confirmed' } })).status()).toBe(200); // confirming is fine
  await api.dispose();

  // the screen does not offer it either
  await page.goto('/admin/admin-citas');
  await page.getByRole('button', { name: 'Lista' }).click();
  const row = page.getByRole('button', { name: new RegExp(`Ver detalles de la cita de Anticipada${id}`) });
  for (let i = 0; i < 3 && !(await row.isVisible()); i++) {
    await page.getByRole('button', { name: 'Siguiente' }).click();
    await page.waitForTimeout(400);
  }
  await row.click();
  const dialog = page.locator(ui.dialog);
  await expect(dialog.getByRole('button', { name: 'Llegó' })).toBeDisabled();
  await expect(dialog.getByRole('button', { name: 'No asistió' })).toBeDisabled();
  await expect(dialog.getByText(/Todavía no es hora/)).toBeVisible();
});
