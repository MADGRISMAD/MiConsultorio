import { apiAsClinic, apiJson, clinicNow, createPersonApi, expect, hhmm, test, ui, uid } from '../support/app';

// Con una cuenta a medias, abrir otra cita pide confirmar en vez de perder el origen; «/» enfoca el catálogo.
test('punto de venta: cuenta en curso pide confirmar y «/» lleva al buscador', async ({ page }) => {
  const id = uid();
  const names = `Curso${id}`;
  const service = `Servicio ${id}`;
  const api = await apiAsClinic();
  await apiJson(await api.post('/api/pos/items', { data: { sku: '', barcode: '', category: '', cost_cents: 0, tax_rate: 0, min_stock: 0, unit: 'pza', kind: 'service', name: service, price_cents: 25000, track_stock: false } }));
  const patient = await createPersonApi(api, names, 'Cobro');
  const pros = (await apiJson(await api.get('/api/agenda/professionals'))).professionals as { id: string; consults: boolean }[];
  const { date, minutes } = clinicNow();
  const start = Math.min(Math.max(0, minutes - 30), 23 * 60);
  const appt = (
    await apiJson(
      await api.post('/api/appointments/', {
        data: {
          patient_id: patient, names, last_names: 'Cobro', CURP: '', date, startHour: hhmm(start), endHour: hhmm(start + 20), details: 'Revisión',
          professional_id: (pros.find((p) => p.consults) ?? pros[0]).id, service_id: null, room: '', phone: '', email: '', reminders_consent: false, overbook: true
        }
      })
    )
  ).appointment;
  await apiJson(await api.post(`/api/appointments/${appt.id}/status`, { data: { status: 'arrived', reason: '', keep_deposit: false } }));
  await api.dispose();

  await page.goto('/pos/cobros');
  await expect(page.getByLabel(/Buscar en el catálogo/)).toBeVisible();
  // «/» enfoca el buscador del catálogo
  await page.getByRole('heading', { name: 'Punto de venta', level: 1 }).click();
  await expect(page.getByLabel(/Buscar en el catálogo/)).not.toBeFocused();
  await page.keyboard.press('/');
  await expect(page.getByLabel(/Buscar en el catálogo/)).toBeFocused();

  // una cuenta a medias
  await page.getByRole('button', { name: new RegExp(service) }).first().click();
  await expect(page.getByTestId('cart-total')).toHaveText('$250.00');

  // pedir la cita pregunta antes de descartar
  await page.getByTestId('today-appointments-panel').getByRole('button', { name: new RegExp(`Cobrar la cita de ${names}`) }).click();
  const dialog = page.locator(ui.dialog);
  await expect(dialog.getByText('Tienes una cuenta sin cobrar')).toBeVisible();
  await dialog.getByRole('button', { name: 'Descartar y abrir' }).click();
  await expect(page.getByTestId('cart-origin')).toContainText(`${names} Cobro`);
});
