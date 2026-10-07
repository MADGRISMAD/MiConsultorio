import { apiAsClinic, apiJson, expect, test, ui, uid } from '../support/app';

test('venta en el punto de venta con caja abierta y cierre de caja', async ({ page }) => {
  const api = await apiAsClinic();
  const id = uid();
  const name = `Consulta ${id}`;
  await apiJson(
    await api.post('/api/pos/items', {
      data: { kind: 'service', name, sku: '', barcode: '', category: '', price_cents: 50000, cost_cents: 0, tax_rate: 0, track_stock: false, min_stock: 0, unit: 'sesión' }
    })
  );
  // start from a closed register whatever an earlier run left behind
  const cur = await apiJson(await api.get('/api/pos/cash/current'));
  if (cur.session) await api.post('/api/pos/cash/close', { data: { counted_cents: cur.session.expected_cents ?? 0, note: '' } });
  await api.dispose();

  await page.goto('/pos/cobros');
  await page.getByRole('button', { name: 'Abrir caja' }).first().click();
  const open = page.locator(ui.dialog);
  await open.locator('input').first().fill('1000');
  await open.locator('button[type=submit]').click();
  await expect(page.getByText('Caja abierta').first()).toBeVisible();

  await page.getByRole('button', { name: new RegExp(name) }).first().click();
  await page.getByRole('button', { name: 'Cobrar' }).first().click();
  const pay = page.locator(ui.dialog);
  await pay.getByLabel('Recibido').fill('700');
  await expect(pay.getByText('$200.00').first()).toBeVisible(); // change
  await pay.locator('button[type=submit]').click();
  await expect(page.getByText('Nueva venta').first()).toBeVisible();

  // the register expects the opening cash plus the sale
  await page.goto('/pos/caja');
  await expect(page.getByText('$1,500.00').first()).toBeVisible();
  await page.getByRole('button', { name: 'Cerrar caja' }).first().click();
  const close = page.locator(ui.dialog);
  await close.locator('input').first().fill('1500');
  await close.locator('button[type=submit]').click();
  await expect(page.getByText('Imprimir corte').first()).toBeVisible();
});
