import { apiAsClinic, apiJson, expect, test, uid } from '../support/app';

// The public booking link: the visitor picks a date, sees the specialists with room that day, the time is held and the clinic is notified.
test('un paciente agenda desde el enlace público: fecha, especialista, horario reservado y aviso a la clínica', async ({ browser }) => {
  const api = await apiAsClinic();
  const slug = `e2e-${uid()}`;
  const pros = (await apiJson(await api.get('/api/agenda/professionals'))).professionals as { id: string; slot_minutes?: number }[];
  await apiJson(await api.put(`/api/agenda/professionals/${pros[0].id}`, { data: { bookable: true, slot_minutes: 30, hours: {}, color: '' } }));
  const st = (await apiJson(await api.get('/api/agenda/settings'))).settings;
  await apiJson(await api.put('/api/agenda/settings', { data: { ...st, booking_enabled: true, booking_slug: slug, booking_lead_hours: 0, booking_horizon_days: 30 } }));

  // a visitor with no session
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await page.goto(`/reservar/${slug}`);
  const day = page.locator('[role=grid] button:not([disabled])').first();
  await expect(day).toBeVisible();
  await day.click();
  await expect(page.getByText('Especialistas disponibles')).toBeVisible();
  await page.locator('input[name=slot]').first().check({ force: true });
  await expect(page.getByText(/Te guardamos este horario/)).toBeVisible();

  const id = uid();
  await page.getByLabel('Nombre(s)').fill(`Visitante${id}`);
  await page.getByLabel('Apellidos').fill('Enlace');
  await page.getByLabel(/Teléfono/).fill('5512345678');
  await page.getByLabel(/Motivo de la cita/).fill('Revisión');
  await page.getByRole('checkbox').first().check();
  await page.getByRole('button', { name: /Agendar cita|Solicitar cita/ }).click();
  await expect(page.getByRole('heading', { name: /agendada|Recibimos tu solicitud/ })).toBeVisible();

  // the clinic gets a notification
  const ntf = await apiJson(await api.get('/api/notifications'));
  expect(JSON.stringify(ntf)).toContain(`Visitante${id}`);
  await ctx.close();
  await api.dispose();
});
