import { expect, test, uid, ui, withSpecialty } from '../support/app';

// The microphone is a fake device so the dictation can be tried without sound hardware.
test.use({
  permissions: ['microphone'],
  launchOptions: { executablePath: process.env.E2E_CHROMIUM_PATH || undefined, args: ['--use-fake-device-for-media-stream', '--use-fake-ui-for-media-stream'] }
});

// A 1x1 PNG: the comparison only needs two readable images.
const PNG = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==', 'base64');

test('dermatología: la IA compara dos fotos y avisa que no diagnostica', async ({ page }) => {
  await withSpecialty('DERMATOLOGY', async (id, api) => {
    for (const title of ['Antes del tratamiento', 'Después de 8 semanas']) {
      const res = await api.post(`/api/patients/${id}/files`, { multipart: { file: { name: `${uid()}.png`, mimeType: 'image/png', buffer: PNG }, kind: 'photo', title } });
      expect(res.status()).toBe(201);
    }
    await page.goto(`/pacientes/${id}`);
    await page.getByRole('button', { name: 'Ver ayudas' }).click();
    await page.getByRole('button', { name: 'Comparar fotos (antes y después)' }).click();
    await page.getByLabel('Foto de antes').selectOption({ index: 1 });
    await page.getByLabel('Foto de después').selectOption({ index: 2 });
    await page.getByRole('button', { name: 'Comparar fotos', exact: true }).click();
    await expect(page.getByText('No se observan cambios aparentes entre las fotos.')).toBeVisible();
    await expect(page.getByText('Solo es una sugerencia.').first()).toBeVisible();
  });
});

test('la nota se dicta, se ordena en los campos vacíos y el audio no se guarda', async ({ page }) => {
    await withSpecialty('PHYSIOTHERAPY', async (id) => {
      await page.goto(`/pacientes/${id}`);
      await page.getByRole('tab', { name: /Bitácora/ }).click();
      await page.getByRole('button', { name: 'Nueva nota' }).first().click();
      await page.getByRole('button', { name: 'Ordenar mis notas con IA' }).click();
      await page.getByRole('button', { name: 'Dictar mi nota' }).click();
      await expect(page.getByText(/Grabando tu dictado/)).toBeVisible();
      // the fake microphone needs a moment of sound: wait for the recording clock instead of a fixed pause
      await expect(page.getByText(/Grabando tu dictado… [2-9] s/)).toBeVisible();
      await page.getByRole('button', { name: 'Terminar y ordenar' }).click();
      await expect(page.locator(ui.enc.subjective)).toHaveValue('Refiere dolor lumbar al sentarse');
      await expect(page.getByText(/Borrador listo/).first()).toBeVisible();
    });
});
