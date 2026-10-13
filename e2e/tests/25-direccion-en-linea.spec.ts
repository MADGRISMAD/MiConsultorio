import { expect, test, uid } from '../support/app';

// The online address is checked while typing: free ones say so; reserved or malformed ones say why and offer variants.
test('la dirección en línea se revisa al escribir y ofrece variantes', async ({ page }) => {
  await page.goto('/ajustes?s=enlinea');
  const input = page.getByLabel('Tu dirección en línea');
  await expect(input).toBeVisible();

  await input.fill('admin');
  await expect(page.getByText('Esa dirección está reservada por la plataforma.')).toBeVisible();
  await expect(page.getByText('Prueba con una de estas, que están libres:')).toBeVisible();
  const first = page.getByRole('button', { name: /^admin-/ }).first();
  await expect(first).toBeVisible();
  await first.click();
  await expect(page.getByText('Disponible')).toBeVisible();

  await input.fill(`mi-consultorio-${uid()}`.toLowerCase());
  await expect(page.getByText('Disponible')).toBeVisible();
});
