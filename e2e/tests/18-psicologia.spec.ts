import { apiAsClinic, apiJson, createPersonApi, expect, test, uid } from '../support/app';

// Psychology: a validated scale is scored by the server and charted; goals and tasks are saved as versions.
test('psicología: escala PHQ-9 con calificación automática y objetivos terapéuticos', async ({ page }) => {
  const api = await apiAsClinic();
  const before = (await apiJson(await api.get('/api/clinic'))).clinic;
  const specialties = [...new Set([...(before.specialties ?? []), 'PSYCHOLOGY'])];
  await apiJson(await api.put('/api/clinic', { data: { specialties } }));
  try {
    const id = await createPersonApi(api, `Psico${uid()}`);
    await page.goto(`/pacientes/${id}`);
    await page.getByRole('tab', { name: 'Escalas y objetivos' }).click();
    await expect(page.getByRole('heading', { name: 'Escalas de evaluación' })).toBeVisible();

    // answer "Varios días" (1) to the nine questions: 9 points, mild
    for (let i = 0; i < 9; i++) await page.getByRole('radiogroup', { name: `Pregunta ${i + 1}` }).getByText('Varios días').click();
    await page.getByRole('button', { name: 'Calcular y guardar' }).click();
    await expect(page.getByText('9 de 27 · Leve').first()).toBeVisible();
    await expect(page.getByText('Revisar pregunta 9').first()).toBeVisible(); // item 9 > 0 is flagged

    await page.getByRole('button', { name: 'Agregar objetivo' }).click();
    await page.getByLabel('Objetivo 1', { exact: true }).fill('Dormir mejor');
    await page.getByRole('button', { name: 'Agregar tarea' }).click();
    await page.getByLabel('Tarea 1', { exact: true }).fill('Registro de pensamientos');
    await page.getByRole('button', { name: 'Guardar objetivos y tareas' }).click();
    await expect(page.getByText('Objetivos y tareas guardados').first()).toBeVisible();
    await page.reload();
    await page.getByRole('tab', { name: 'Escalas y objetivos' }).click();
    await expect(page.getByLabel('Objetivo 1', { exact: true })).toHaveValue('Dormir mejor');
  } finally {
    await api.put('/api/clinic', { data: { specialties: before.specialties ?? [] } });
    await api.dispose();
  }
});
