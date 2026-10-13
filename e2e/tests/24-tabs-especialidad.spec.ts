import { expect, test, withSpecialty } from '../support/app';

// Each giro adds one specialty to the clinic (the plan allows three in all), opens the patient and saves a version.
test('odontología: el periodontograma marca las bolsas y se guarda', async ({ page }) => {
  await withSpecialty('DENTAL', async (id) => {
    await page.goto(`/pacientes/${id}`);
    await page.getByRole('tab', { name: /Periodontograma/ }).click();
    await page.getByRole('button', { name: 'Diente 16', exact: true }).click();
    await page.getByLabel('Vestibular, Medio: profundidad al sondaje').fill('5');
    await page.getByLabel('Vestibular, Distal: profundidad al sondaje').fill('3');
    await page.getByLabel('Vestibular, Medio: sangrado al sondaje').check();
    await page.getByRole('button', { name: 'Guardar medición' }).click();
    await expect(page.getByText('Periodontograma guardado').first()).toBeVisible();
    await expect(page.getByText('1 bolsas ≥5 mm').first()).toBeVisible();
  });
});

test('nutrición: el recordatorio de 24 horas se guarda', async ({ page }) => {
  await withSpecialty('NUTRITION', async (id) => {
    await page.goto(`/pacientes/${id}`);
    await page.getByRole('tab', { name: /Recordatorio 24 h/ }).click();
    await page.getByLabel('Qué comió y cuánto').first().fill('2 huevos con frijoles y 2 tortillas');
    await page.getByRole('button', { name: 'Guardar recordatorio' }).click();
    await expect(page.getByText('Recordatorio guardado').first()).toBeVisible();
  });
});

test('fisioterapia: la hoja de ejercicios se guarda', async ({ page }) => {
  await withSpecialty('PHYSIOTHERAPY', async (id) => {
    await page.goto(`/pacientes/${id}`);
    await page.getByRole('tab', { name: /Hoja de ejercicios/ }).click();
    await page.getByRole('button', { name: 'Agregar ejercicio' }).click();
    await page.getByLabel('Ejercicio 1', { exact: true }).fill('Puente de glúteo');
    await page.getByRole('button', { name: 'Guardar hoja' }).click();
    await expect(page.getByText('Hoja de ejercicios guardada').first()).toBeVisible();
  });
});
