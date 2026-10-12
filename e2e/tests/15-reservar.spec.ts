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
  await page.goto(`/${slug}/reservar`);
  // the details come first: the calendar stays closed until they are filled in
  await expect(page.getByText('Completa tus datos para elegir la fecha.')).toBeVisible();
  const id = uid();
  await page.getByLabel('Nombre completo').fill(`Visitante${id} Enlace`);
  await page.getByLabel(/Teléfono/).fill('5512345678');
  await page.getByLabel('Correo').fill(`v${id}@correo.mx`);
  // a clinic with several areas asks which one the visit is for
  await page.getByRole('radio', { name: 'Medicina general' }).check({ force: true });
  const day = page.locator('[role=grid] button:not([disabled])').first();
  await expect(day).toBeVisible();
  await day.click();
  await expect(page.getByText('Especialistas disponibles')).toBeVisible();
  await page.locator('input[name=slot]').first().check({ force: true });
  await expect(page.getByText(/Te guardamos este horario/)).toBeVisible();

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

test('el enlace viejo /reservar/<nombre> lleva al nuevo /<nombre>/reservar', async ({ page }) => {
  await page.goto('/reservar/enlace-viejo-x');
  await expect(page).toHaveURL(/\/enlace-viejo-x\/reservar$/);
});

test('un servicio por videollamada: el paciente elige la modalidad y recibe su enlace', async ({ browser }) => {
  const api = await apiAsClinic();
  const slug = `e2e-${uid()}`;
  const pros = (await apiJson(await api.get('/api/agenda/professionals'))).professionals as { id: string }[];
  await apiJson(await api.put(`/api/agenda/professionals/${pros[0].id}`, { data: { bookable: true, slot_minutes: 30, hours: {}, color: '' } }));
  const st = (await apiJson(await api.get('/api/agenda/settings'))).settings;
  await apiJson(await api.put('/api/agenda/settings', { data: { ...st, booking_enabled: true, booking_slug: slug, booking_lead_hours: 0, booking_horizon_days: 30 } }));
  const name = `Seguimiento ${uid()}`;
  await apiJson(await api.post('/api/pos/items', { data: { kind: 'service', name, price_cents: 50000, modality: 'ambas' } }));

  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await page.goto(`/${slug}/reservar`);
  const id = uid();
  await page.getByLabel('Nombre completo').fill(`Video${id} Visitante`);
  await page.getByLabel(/Teléfono/).fill(`55${String(Date.now()).slice(-8)}`); // otro teléfono: una cita abierta por giro y paciente
  await page.getByLabel('Correo').fill(`v${id}@correo.mx`);
  const area = page.getByRole('radio', { name: 'Medicina general' });
  if (await area.count()) await area.check({ force: true });
  await page.getByText(name).click();
  // the service gives both: the visitor chooses
  await expect(page.getByText('Modalidad')).toBeVisible();
  await page.getByText('Videollamada', { exact: true }).click();
  const day = page.locator('[role=grid] button:not([disabled])').first();
  await day.click();
  await page.locator('input[name=slot]').first().check({ force: true });
  await page.getByLabel(/Motivo de la cita/).fill('Revisión');
  await page.getByRole('checkbox').first().check();
  await page.getByRole('button', { name: /Agendar cita|Solicitar cita/ }).click();
  await expect(page.getByRole('heading', { name: /agendada|Recibimos tu solicitud/ })).toBeVisible();
  await expect(page.getByText('Enlace de tu videollamada')).toBeVisible();
  await expect(page.getByRole('link', { name: /Unirme a la videollamada/ })).toHaveAttribute('href', /https:\/\/.+/);
  await ctx.close();
  await api.dispose();
});
