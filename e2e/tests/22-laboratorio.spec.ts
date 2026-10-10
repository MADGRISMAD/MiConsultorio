import { apiAsClinic, apiJson, createPersonApi, expect, test, uid } from '../support/app';

// Laboratory tab: orders open and close, the sheet for the laboratory is printable and a report can be scanned with AI.
test('las órdenes de laboratorio se contraen y se pueden escanear o imprimir', async ({ page }) => {
  const api = await apiAsClinic();
  const id = await createPersonApi(api, `Lab${uid()}`);
  const mk = async (title: string, extra: object = {}) => apiJson(await api.post(`/api/patients/${id}/lab/orders`, { data: { title, ...extra } }));
  await mk('Chequeo antiguo', { requested: ['Biometría hemática'] });
  await mk('Química reciente', {
    requested: ['Química sanguínea de 12 elementos'],
    results: [{ panel: 'Química', analyte: 'Glucosa en ayuno', value_num: 130, unit: 'mg/dL', ref_low: 70, ref_high: 99 }]
  });
  await api.dispose();

  await page.goto(`/pacientes/${id}`);
  await page.getByRole('tab', { name: /Laboratorio/ }).click();
  const newest = page.getByRole('button', { name: /Química reciente/ });
  const oldest = page.getByRole('button', { name: /Chequeo antiguo/ });
  await expect(newest).toHaveAttribute('aria-expanded', 'true');
  await expect(oldest).toHaveAttribute('aria-expanded', 'false');
  await expect(page.getByText('1 estudio solicitado')).toBeVisible(); // the closed one still says what it is
  await expect(page.getByRole('button', { name: /Imprimir orden para el laboratorio/ })).toHaveCount(1);

  await oldest.click();
  await expect(oldest).toHaveAttribute('aria-expanded', 'true');
  await expect(page.getByText('Estudios solicitados: Biometría hemática')).toBeVisible();
  await page.getByRole('button', { name: 'Contraer todo' }).click();
  await expect(newest).toHaveAttribute('aria-expanded', 'false');
  await expect(page.getByText('1 fuera de rango')).toBeVisible();

  await page.getByRole('button', { name: 'Escanear resultados' }).click();
  await expect(page.getByRole('heading', { name: 'Escanear resultados con IA' })).toBeVisible();
  await expect(page.getByText('Revisa siempre cada valor')).toBeVisible();
});
