import { apiAsClinic, createPersonApi, expect, test, uid } from '../support/app';

// Every screen that loads its data opens without a script error and without an error notice.
// (They share one loading/error helper: this catches a screen that stopped loading.)
const ROUTES = [
  '/pos/cobros',
  '/pos/caja',
  '/pos/inventario',
  '/pos/facturacion',
  '/pos/cuentas',
  '/pos/reportes',
  '/pos/servicios',
  '/equipo',
  '/arco-solicitudes',
  '/en-proceso',
  '/pacientes',
  '/agenda/espera',
  '/ajustes?s=seguridad',
  '/ajustes?s=cumplimiento',
  '/ajustes?s=crecimiento',
  '/ajustes?s=agenda',
  '/ajustes?s=terminal',
  '/ajustes?s=perfil',
  '/indicadores'
];

for (const route of ROUTES) {
  test(`la pantalla ${route} carga sin errores`, async ({ page }) => {
    const errors: string[] = [];
    page.on('pageerror', (e) => errors.push(e.message));
    await page.goto(route);
    await page.waitForLoadState('networkidle');
    await expect(page.locator('main, [role=main], body').first()).toBeVisible();
    await expect(page.locator('[role=alert]:visible')).toHaveCount(0);
    expect(errors).toEqual([]);
  });
}

test('el expediente de un paciente abre todas sus pestañas sin errores', async ({ page }) => {
  const api = await apiAsClinic();
  const id = await createPersonApi(api, `Pantalla${uid()}`);
  await api.dispose();
  const errors: string[] = [];
  page.on('pageerror', (e) => errors.push(e.message));
  await page.goto(`/pacientes/${id}`);
  await page.waitForLoadState('networkidle');
  const tabs = page.getByRole('tab');
  const n = await tabs.count();
  expect(n).toBeGreaterThan(2);
  for (let i = 0; i < n; i++) {
    await tabs.nth(i).click();
    await page.waitForLoadState('networkidle');
    await expect(page.locator('[role=alert]:visible')).toHaveCount(0);
  }
  expect(errors).toEqual([]);
});

test('Ajustes tiene un solo botón Guardar que acompaña el scroll y guarda todo el apartado', async ({ page }) => {
  await page.goto('/ajustes?s=negocio');
  await page.waitForLoadState('networkidle');
  const guardar = page.getByRole('region', { name: 'Guardar cambios' }).getByRole('button', { name: 'Guardar', exact: true });
  await expect(guardar).toBeVisible();
  // los botones propios de cada bloque ya no aparecen
  await expect(page.getByRole('button', { name: /^Guardar (datos|giro|horario)$/ })).toHaveCount(0);
  await page.mouse.wheel(0, 3000);
  await expect(guardar).toBeInViewport();
  await page.getByLabel(/^Teléfono/).first().fill('5512345678');
  await guardar.click();
  await expect(page.getByText('Datos guardados')).toBeVisible();
  await expect(page.getByText('Horario guardado')).toBeVisible();
});
