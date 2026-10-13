import { apiAsClinic, apiJson, createPersonApi, expect, test, uid } from '../support/app';

// The AI helpers answer with drafts and suggestions that land in editable boxes (a fake Gemini answers in the stack).
async function withSpecialty(kind: string, run: (patientId: string) => Promise<void>) {
  const api = await apiAsClinic();
  const before = (await apiJson(await api.get('/api/clinic'))).clinic;
  await apiJson(await api.put('/api/clinic', { data: { specialties: [kind] } }));
  try {
    await run(await createPersonApi(api, `Ia${uid()}`));
  } finally {
    await api.put('/api/clinic', { data: { specialties: before.specialties ?? [] } });
    await api.dispose();
  }
}

test('fisioterapia: la IA sugiere ejercicios y el especialista los edita a mano', async ({ page }) => {
  await withSpecialty('PHYSIOTHERAPY', async (id) => {
    await page.goto(`/pacientes/${id}`);
    await page.getByRole('tab', { name: /Hoja de ejercicios/ }).click();
    await page.getByRole('button', { name: 'Sugerir ejercicios con IA' }).click();
    await expect(page.getByText('Es solo una sugerencia.')).toBeVisible();
    await page.getByLabel('Zona a tratar').fill('Lumbar');
    await page.getByRole('button', { name: 'Sugerir ejercicios', exact: true }).click();
    // the draft lands in the editable sheet
    const first = page.getByLabel('Ejercicio 1', { exact: true });
    await expect(first).toHaveValue('Puente de glúteo');
    await first.fill('Puente de glúteo con banda');
    await page.getByRole('button', { name: 'Guardar hoja' }).click();
    await expect(page.getByText('Hoja de ejercicios guardada').first()).toBeVisible();
  });
});

test('la nota de consulta: la IA ordena las notas sueltas en los campos vacíos', async ({ page }) => {
  await withSpecialty('PHYSIOTHERAPY', async (id) => {
    await page.goto(`/pacientes/${id}`);
    await page.getByRole('tab', { name: /Bitácora/ }).click();
    await page.getByRole('button', { name: 'Nueva nota' }).first().click();
    await page.getByRole('button', { name: 'Ordenar mis notas con IA' }).click();
    await page.getByLabel('Tus notas sueltas').fill('Refiere dolor lumbar al sentarse, EVA 6, contractura. Calor y ejercicios.');
    await page.getByRole('button', { name: 'Ordenar en la nota' }).click();
    await expect(page.locator('#enc-subj')).toHaveValue('Refiere dolor lumbar al sentarse');
    await expect(page.locator('#enc-ass')).toHaveValue('Lumbalgia mecánica');
    await expect(page.getByText(/Borrador listo/).first()).toBeVisible();
  });
});

test('las ayudas con IA del paciente: un mensaje llega como borrador con aviso', async ({ page }) => {
  await withSpecialty('NUTRITION', async (id) => {
    await page.goto(`/pacientes/${id}`);
    await page.getByRole('button', { name: 'Ver ayudas' }).click();
    await page.getByRole('button', { name: 'Mensaje para el paciente' }).click();
    await page.getByLabel('Tipo de mensaje').selectOption('Seguimiento');
    await page.getByRole('button', { name: 'Generar' }).click();
    await expect(page.getByText('Borrador.', { exact: false }).first()).toBeVisible();
    await expect(page.getByText('Hola [nombre]').first()).toBeVisible();
    await expect(page.getByRole('button', { name: 'Imprimir', exact: true })).toBeVisible();
  });
});
