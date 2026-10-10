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

test('«Nueva orden» solo solicita los estudios y detalla qué se pide', async ({ page }) => {
  const api = await apiAsClinic();
  const id = await createPersonApi(api, `Orden${uid()}`);
  await api.dispose();
  await page.goto(`/pacientes/${id}`);
  await page.getByRole('tab', { name: /Laboratorio/ }).click();
  await page.getByRole('button', { name: 'Nueva orden', exact: true }).first().click();
  const dlg = page.getByRole('dialog', { name: 'Nueva orden de laboratorio' });
  await expect(dlg).toBeVisible();
  // no result fields: this window only asks for studies
  await expect(dlg.getByText('Fecha del resultado')).toHaveCount(0);
  await expect(dlg.getByText('Con esto la orden queda completa')).toHaveCount(0);
  await expect(dlg.getByRole('button', { name: 'Guardar orden' })).toBeDisabled();
  await dlg.getByLabel('Estudios que solicitas').selectOption({ label: 'Química sanguínea de 12 elementos' });
  await expect(dlg.getByText(/Incluye \d+ análisis/)).toBeVisible();
  await expect(dlg.getByText('Glucosa en ayuno')).toBeVisible();
  await dlg.getByLabel('Otro estudio (si no está en la lista)').fill('Perfil tiroideo');
  await dlg.getByRole('button', { name: 'Agregar', exact: true }).click();
  await dlg.getByLabel('Indicaciones para el paciente (opcional)').fill('Ayuno de 8 horas');
  await dlg.getByRole('button', { name: 'Guardar orden' }).click();
  await expect(page.getByText('Orden guardada')).toBeVisible();
  await expect(page.getByText('Estudios solicitados: Química sanguínea de 12 elementos · Perfil tiroideo')).toBeVisible();
  await expect(page.getByText('Esperando los resultados del paciente')).toBeVisible();
});
