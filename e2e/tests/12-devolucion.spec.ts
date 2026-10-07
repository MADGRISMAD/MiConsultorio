import { apiAsClinic, apiJson, expect, test, ui, uid } from '../support/app';

test('devolución parcial de un producto: regresa al inventario y sale de la caja', async ({ page }) => {
  const api = await apiAsClinic();
  const id = uid();
  const product = `Gasas ${id}`;
  const customer = `Cliente ${id}`;
  const item = await apiJson(
    await api.post('/api/pos/items', {
      data: { kind: 'product', name: product, sku: '', barcode: '', category: '', price_cents: 10000, cost_cents: 4000, tax_rate: 0, track_stock: true, stock: 5, min_stock: 0, unit: 'pza' }
    })
  );
  const cur = await apiJson(await api.get('/api/pos/cash/current'));
  if (!cur.session) await apiJson(await api.post('/api/pos/cash/open', { data: { opening_cents: 0 } }));
  await apiJson(
    await api.post('/api/pos/sales', {
      data: { lines: [{ item_id: item.item.id, qty: 3 }], customer_name: customer, payments: [{ method: 'cash', amount_cents: 30000, received_cents: 30000 }] }
    })
  );

  await page.goto('/pos/reportes');
  await page.getByText(customer).first().click();
  await page.getByRole('button', { name: 'Devolución' }).first().click();
  const dialog = page.locator(ui.dialog).last();
  await dialog.getByLabel(new RegExp(`Cantidad a devolver de ${product}`)).fill('1');
  await dialog.getByLabel('Motivo').fill('Vino dañada');
  await expect(dialog.getByText('$100.00').first()).toBeVisible();
  await dialog.getByRole('button', { name: 'Registrar devolución' }).click();
  await expect(dialog.getByText('Devolución registrada').first()).toBeVisible();
  await expect(dialog.getByText('sale de la caja').first()).toBeVisible();
  await dialog.getByRole('button', { name: 'Listo' }).click();

  // one unit is back on the shelf and the sale shows what came back
  const items = await apiJson(await api.get(`/api/pos/items?q=${encodeURIComponent(product)}`));
  expect(items.items[0].stock).toBe(3); // 5 - 3 sold + 1 returned
  await expect(page.getByText('Devuelto').first()).toBeVisible();
  await api.dispose();
});
