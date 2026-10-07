import { apiAsClinic, apiJson, expect, test, ui, uid } from '../support/app';

test('solicitud de factura de una venta', async ({ page }) => {
  const api = await apiAsClinic();
  const id = uid();
  const customer = `Cliente ${id}`;
  // a paid sale to invoice (register opened and closed around it through the API)
  const cur = await apiJson(await api.get('/api/pos/cash/current'));
  if (!cur.session) await apiJson(await api.post('/api/pos/cash/open', { data: { opening_cents: 0 } }));
  await apiJson(
    await api.post('/api/pos/sales', {
      data: {
        lines: [{ name: 'Certificado médico', qty: 1, unit_price_cents: 30000 }],
        customer_name: customer,
        payments: [{ method: 'cash', amount_cents: 30000, received_cents: 30000 }]
      }
    })
  );
  await api.dispose();

  await page.goto('/pos/facturacion');
  await page.getByRole('button', { name: 'Nueva solicitud' }).first().click();
  const dialog = page.locator(ui.dialog);
  await dialog.getByLabel('Buscar venta').fill(customer);
  await dialog.getByRole('button', { name: new RegExp(customer) }).first().click();
  await dialog.locator(ui.inv.rfc).fill('XAXX010101000');
  await dialog.locator(ui.inv.zip).fill('06600');
  await dialog.locator(ui.inv.name).fill('Público en general');
  await dialog.locator(ui.inv.regime).selectOption({ index: 1 });
  await dialog.locator(ui.inv.use).selectOption({ index: 1 });
  await dialog.locator('button[type=submit]').click();
  await expect(dialog).toBeHidden();
  await expect(page.getByText('XAXX010101000').first()).toBeVisible();
});
