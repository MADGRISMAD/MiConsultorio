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
  '/ajustes?s=perfil'
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
