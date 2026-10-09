import { apiAsClinic, apiJson, createPersonApi, expect, test, uid } from '../support/app';

test('el resumen del paciente grafica la evolución de peso y presión con dos o más mediciones', async ({ page }) => {
  const api = await apiAsClinic();
  const id = await createPersonApi(api, `Evolucion${uid()}`);
  const enc = (measures: Record<string, number>, at: string) =>
    api.post(`/api/patients/${id}/encounters`, { data: { reason: 'Control', occurred_at: at, measures } });
  await apiJson(await enc({ weight_kg: 80, bp_sys: 140, bp_dia: 90 }, new Date(Date.now() - 6 * 864e5).toISOString()));
  await apiJson(await enc({ weight_kg: 77, bp_sys: 130, bp_dia: 84 }, new Date(Date.now() - 1 * 864e5).toISOString()));
  await api.dispose();

  await page.goto(`/pacientes/${id}`);
  await expect(page.getByRole('heading', { name: 'Evolución' })).toBeVisible();
  await expect(page.getByText('Presión arterial').first()).toBeVisible();
  await expect(page.locator('svg[role=img]').first()).toBeVisible();
});
