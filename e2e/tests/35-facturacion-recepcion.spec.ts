import { apiAsClinic, apiJson, expect, test, uid } from '../support/app';

// Recepción cobra pero no revisa facturas: su pantalla solo captura solicitudes y no pide una lista prohibida.
test('facturación: recepción ve la captura de solicitudes sin errores de acceso', async ({ browser }) => {
  const id = uid();
  const username = `recep${id}`;
  const password = `Recep-${id}-123`;
  const api = await apiAsClinic();
  await apiJson(await api.post('/api/team', { data: { name: `Recepción ${id}`, email: `${username}@e2e.test`, username, phone: '', password, role: 'reception', areas: [] } }));
  await api.dispose();

  const context = await browser.newContext({ storageState: { cookies: [], origins: [] }, locale: 'es-MX' });
  const page = await context.newPage();
  const denied: string[] = [];
  page.on('response', (r) => r.status() === 403 && denied.push(r.url()));
  await page.goto('/login');
  await page.getByLabel(/correo|usuario/i).first().fill(username);
  await page.locator('input[type=password]').fill(password);
  await page.keyboard.press('Enter');
  await page.waitForURL((u) => !u.pathname.startsWith('/login'));

  await page.goto('/pos/facturacion');
  await expect(page.getByRole('heading', { name: 'Solicita la factura de una venta' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Nueva solicitud' }).first()).toBeVisible();
  await expect(page.getByText('No se pudieron cargar las solicitudes.')).toHaveCount(0);
  expect(denied.filter((u) => u.includes('/api/invoices'))).toEqual([]);
  await context.close();
});
