import { expect, test } from '../support/app';

test('la pantalla de reportes carga y exporta', async ({ page }) => {
  await page.goto('/reportes');
  await expect(page.getByRole('heading', { name: 'Reportes' })).toBeVisible();
  await expect(page.getByText('Consultas').first()).toBeVisible();
  await expect(page.getByRole('link', { name: /Exportar CSV/ })).toBeVisible();

  await page.getByRole('tab', { name: 'Pacientes' }).click();
  await expect(page.getByText('Pacientes atendidos')).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Motivos de consulta más frecuentes' })).toBeVisible();
});
