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
  const row = page.getByRole('region', { name: 'Ahora', exact: true });
  await expect(row).toBeVisible();
  await expect(row.getByText(names + ' Consulta', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Iniciar consulta', exact: true })).toHaveCount(1);

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
  await page.getByText(/^Atendidas \(/).click();
  await expect(page.getByRole('listitem').filter({ hasText: names }).getByText('Completada')).toBeVisible();
});

test('inicio móvil: disponibilidad clara, búsqueda por teclado y sin banner sobre las acciones', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.addInitScript(() => localStorage.removeItem('caresia-cookies'));
  await page.route('**/api/appointments/?*', async (route) => {
    const date = new URL(route.request().url()).searchParams.get('from');
    await route.fulfill({ json: { appointments: [{
      id: 'inicio-futuro', patient_id: 'paciente-prueba', names: 'Paciente', last_names: 'Futuro', date,
      startHour: '23:50', endHour: '23:59', status: 'scheduled', details: 'Revisión',
      professional_id: null, professional_name: '', service_name: '',
      opens_at: new Date(Date.now() + 3600000).toISOString()
    }] } });
  });
  await page.route('**/api/patients/lookup?*', async (route) => {
    const q = new URL(route.request().url()).searchParams.get('q');
    if (q === 'error') return route.fulfill({ status: 500, json: { error: 'Error de prueba' } });
    await route.fulfill({ json: { patients: q === 'sin resultados' ? [] : [{ id: 'paciente-prueba', names: 'Ana', last_names: 'Prueba', file_number: 42, phone: '5512345678' }] } });
  });
  await page.goto('/');
  const now = page.getByRole('region', { name: 'Ahora', exact: true });
  await expect(now.getByText(/Disponible a las/)).toBeVisible();
  await expect(now.getByRole('button', { name: 'Iniciar consulta' })).toHaveCount(0);
  const record = now.getByRole('link', { name: 'Expediente' });
  await expect(record).toBeInViewport();
  await expect(page.getByRole('link', { name: 'Consulta sin cita', exact: true })).toBeVisible();
  await expect(page.getByRole('dialog', { name: 'Cookies en este sitio' })).toHaveCount(0);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);

  await page.keyboard.press('/');
  const search = page.getByRole('combobox', { name: 'Buscar paciente' });
  await expect(search).toBeFocused();
  await search.fill('sin resultados');
  await expect(page.getByRole('status').filter({ hasText: 'Sin resultados' })).toBeVisible();
  await search.fill('error');
  await expect(page.getByText('No pudimos buscar. Intenta de nuevo.')).toBeVisible();
  await search.fill('Ana');
  await expect(page.getByRole('option')).toHaveCount(1);
  await search.press('ArrowUp');
  await expect(page.getByRole('option')).toHaveAttribute('aria-selected', 'true');
  await search.press('Escape');
  await expect(search).toHaveAttribute('aria-expanded', 'false');
  await search.press('Control+k');
  await expect(search).toBeFocused();
  await search.press('ArrowDown');
  await search.press('Enter');
  await page.waitForURL('**/pacientes/paciente-prueba');
});
