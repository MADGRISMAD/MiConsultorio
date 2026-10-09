import { apiAsClinic, apiJson, expect, test, ui, uid } from '../support/app';

// Mixed payment: part on the clinic's own card terminal, the rest in cash with change, in one step.
test('cobro mixto: parte con tarjeta de la terminal propia y el resto en efectivo', async ({ page }) => {
  const api = await apiAsClinic();
  const name = `Consulta mixta ${uid()}`;
  await apiJson(
    await api.post('/api/pos/items', {
      data: { kind: 'service', name, sku: '', barcode: '', category: '', price_cents: 50000, cost_cents: 0, tax_rate: 0, track_stock: false, min_stock: 0, unit: 'sesión' }
    })
  );
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
  await pay.getByRole('radio', { name: /Mixto/ }).click();
  await pay.getByLabel(/Parte con tarjeta/).fill('200');
  await expect(pay.getByText('$300.00').first()).toBeVisible(); // the rest, in cash
  await pay.getByLabel('Referencia del voucher (opcional)').fill('4242');
  await pay.getByLabel('Efectivo recibido').fill('500');
  await expect(pay.getByText('$200.00').first()).toBeVisible(); // change
  await pay.locator('button[type=submit]').click();
  await expect(page.getByText('Nueva venta').first()).toBeVisible();

  // only the cash part enters the register: opening cash plus 300
  await page.goto('/pos/caja');
  await expect(page.getByText('$1,300.00').first()).toBeVisible();
  await page.getByRole('button', { name: 'Cerrar caja' }).first().click();
  const close = page.locator(ui.dialog);
  await close.locator('input').first().fill('1300');
  await close.locator('button[type=submit]').click();
  await expect(page.getByText('Imprimir corte').first()).toBeVisible();
});
