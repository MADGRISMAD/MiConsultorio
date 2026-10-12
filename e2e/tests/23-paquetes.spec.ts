import { apiAsClinic, apiJson, expect, test, uid } from '../support/app';

// Session packages: the specialist creates one, it is listed, and it is edited.
test('paquetes de sesiones: se crea, se lista con el precio por sesión y se edita', async ({ page }) => {
  const api = await apiAsClinic();
  const before = (await apiJson(await api.get('/api/clinic'))).clinic;
  const specialties = [...new Set([...(before.specialties ?? []), 'PSYCHOLOGY'])];
  await apiJson(await api.put('/api/clinic', { data: { specialties } }));
  try {
    const name = `Terapia ×5 ${uid()}`;
    await page.goto('/agenda/paquetes');
    await expect(page.getByRole('heading', { name: 'Paquetes de sesiones' })).toBeVisible();
    await page.getByRole('button', { name: 'Nuevo paquete' }).click();
    await page.getByLabel('Nombre').fill(name);
    await page.getByLabel('Precio total (MXN)').fill('3500');
    await page.getByLabel('Número de sesiones').fill('5');
    await page.getByRole('button', { name: 'Guardar' }).click();
    const card = page.getByRole('listitem').filter({ hasText: name });
    await expect(card).toContainText('5 sesiones');
    await expect(card).toContainText('$700.00 cada una');

    await card.getByRole('button', { name: 'Editar' }).click();
    await page.getByLabel('Precio total (MXN)').fill('3000');
    await page.getByRole('button', { name: 'Guardar' }).click();
    await expect(card).toContainText('$600.00 cada una');
  } finally {
    await api.put('/api/clinic', { data: { specialties: before.specialties ?? [] } });
    await api.dispose();
  }
});
