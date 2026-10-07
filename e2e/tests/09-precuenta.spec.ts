import { apiAsClinic, apiJson, createPersonApi, expect, test, ui, uid } from '../support/app';

test('consulta con pre-cuenta, caja y cobro sin descontar dos veces el insumo', async ({ page }) => {
  const api = await apiAsClinic();
  const id = uid();
  const service = `Curación ${id}`;
  const supply = `Gasa ${id}`;
  const item = (extra: object) => ({ sku: '', barcode: '', category: '', cost_cents: 0, tax_rate: 0, min_stock: 0, unit: 'pza', ...extra });
  await apiJson(await api.post('/api/pos/items', { data: item({ kind: 'service', name: service, price_cents: 50000, track_stock: false }) }));
  const gasa = (await apiJson(await api.post('/api/pos/items', { data: item({ kind: 'product', name: supply, price_cents: 2000, track_stock: true, stock: 10 }) }))).item.id;
  const cur = await apiJson(await api.get('/api/pos/cash/current'));
  if (!cur.session) await apiJson(await api.post('/api/pos/cash/open', { data: { opening_cents: 0 } }));
  const patient = await createPersonApi(api, `Precuenta${id}`, 'Paciente');

  // the professional builds the pre-account inside the consultation and sends it to the register
  await page.goto(`/pacientes/${patient}`);
  await page.getByRole('tab', { name: /Bitácora/ }).click();
  await page.getByRole('button', { name: 'Nueva nota' }).first().click();
  await page.fill(ui.enc.reason, 'Curación de herida');
  await page.fill('#enc-form-charge-svc', service);
  await page.locator('#enc-form-charge-svc-list').getByRole('button', { name: new RegExp(service) }).click();
  await page.fill('#enc-form-charge-sup-pick', supply);
  await page.locator('#enc-form-charge-sup-pick-list').getByRole('button', { name: new RegExp(supply) }).click();
  await expect(page.getByText('Existencias vigentes: 10')).toBeVisible();
  await page.getByLabel(`Cantidad de ${supply}`).fill('3');
  await expect(page.getByText('$560.00').first()).toBeVisible();
  await page.getByRole('button', { name: 'Guardar en la bitácora' }).click();
  await expect(page.getByText('Pre-cuenta enviada a caja').first()).toBeVisible();
  await expect(page.getByText('Enviada a caja · $560.00')).toBeVisible();

  // the supply left the inventory when the consultation was saved
  const stock = async () => (await apiJson(await api.get(`/api/pos/items?q=${encodeURIComponent(supply)}`))).items[0].stock;
  expect(await stock()).toBe(7);

  // the register sees it and opens the charge with one click
  await page.goto('/pos/cobros');
  const panel = page.getByTestId('consult-charges-panel');
  await expect(panel.getByText(`Precuenta${id} Paciente`)).toBeVisible();
  await panel.getByRole('button', { name: /Cobrar la pre-cuenta de/ }).first().click();
  await expect(page.getByText('Pre-cuenta de la consulta').first()).toBeVisible();
  await expect(page.getByText(service).first()).toBeVisible();
  await page.getByRole('button', { name: /^Cobrar \$560/ }).first().click();
  const pay = page.locator(ui.dialog);
  await pay.getByLabel('Recibido').fill('600');
  await expect(pay.getByText('$40.00').first()).toBeVisible();
  await pay.locator('button[type=submit]').click();
  await expect(page.getByText('Nueva venta').first()).toBeVisible();

  // charged once, stock untouched by the sale, nothing left to charge
  expect(await stock()).toBe(7);
  const left = await apiJson(await api.get('/api/consult-charges?status=sent'));
  expect(left.charges.filter((c: any) => c.patient_id === patient)).toHaveLength(0);
  const done = await apiJson(await api.get(`/api/consult-charges?patient_id=${patient}`));
  expect(done.charges[0].status).toBe('charged');
  expect(gasa).toBeTruthy();
  await api.dispose();
});
