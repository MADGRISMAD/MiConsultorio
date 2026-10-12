import { apiAsClinic, apiJson, expect, test, uid } from '../support/app';

// «Reservas y página pública»: la dirección se define una vez, aquí mismo, y de ahí cuelgan las reservas,
// la página, el directorio y el portal. Sin dar vueltas por otras secciones.
test('de la dirección a la página publicada, sin salir de «Reservas y página pública»', async ({ page }) => {
  const slug = `presencia-${uid()}`;
  // otras pruebas dejan una dirección puesta: se parte de una clínica que aún no tiene
  const api = await apiAsClinic();
  const st = (await apiJson(await api.get('/api/agenda/settings'))).settings;
  await apiJson(await api.put('/api/agenda/settings', { data: { ...st, booking_enabled: false, booking_slug: '' } }));
  await page.goto('/ajustes?s=enlinea');
  const tab = (name: string) => page.getByRole('tab', { name: new RegExp(`^0\\d\\s.*${name}`) });

  // los cinco pasos, con su estado
  for (const n of ['Reservas', 'Página', 'Directorio', 'Paciente', 'Opiniones']) await expect(tab(n)).toBeVisible();

  // 1. sin dirección no se puede encender nada, y el aviso lleva de vuelta a «Reservas»
  await tab('Página').click();
  await expect(page.getByRole('checkbox', { name: /Publicar la página/ })).toBeDisabled();
  await page.getByRole('button', { name: /Ir a «Reservas»/ }).first().click();
  await expect(tab('Reservas')).toHaveAttribute('aria-selected', 'true');

  // 2. la dirección y encender las reservas
  await page.getByRole('textbox', { name: 'Tu dirección en línea' }).fill(slug);
  await page.getByRole('checkbox', { name: /reserven en línea/ }).check();
  await page.getByRole('button', { name: 'Guardar reservas' }).click();
  await expect(page.getByText('Reservas en línea guardadas')).toBeVisible();
  await expect(tab('Reservas')).toContainText('Activas');
  await expect(page.getByRole('button', { name: 'Copiar enlace' })).toBeVisible();

  // 3. la página ya se puede publicar con esa dirección, sin recargar
  await tab('Página').click();
  const publicar = page.getByRole('checkbox', { name: /Publicar la página/ });
  await expect(publicar).toBeEnabled();
  await publicar.check();
  await page.getByRole('button', { name: 'Guardar cambios' }).click();
  await expect(page.getByText('Perfil y encuesta guardados')).toBeVisible();
  await expect(tab('Página')).toContainText('Publicada');

  // 4. el directorio (automático) y el portal
  await tab('Directorio').click();
  // todos salen en el directorio: aquí se ve el puntaje del perfil y qué falta para subir
  await expect(page.getByRole('progressbar', { name: /qué tan completo/i })).toBeVisible();
  await expect(page.getByText('Para subir, te falta:')).toBeVisible();
  await expect(tab('Directorio')).toContainText(/Perfil \d+\/100/);
  await expect(page.getByRole('checkbox', { name: /No mostrar mi consultorio/ })).not.toBeChecked();
  await tab('Paciente').click();
  await expect(page.getByRole('checkbox', { name: /Activar el portal/ })).toBeEnabled();

  // el paso elegido queda en la dirección: se puede compartir o recargar
  await expect(page).toHaveURL(/t=portal/);
  await page.reload();
  await expect(tab('Paciente')).toHaveAttribute('aria-selected', 'true');
});

test('«Agenda» ya no mezcla lo público: avisa dónde están las reservas', async ({ page }) => {
  await page.goto('/ajustes?s=agenda');
  await expect(page.getByText('Citas y salas')).toBeVisible();
  await expect(page.getByRole('textbox', { name: 'Tu dirección en línea' })).toHaveCount(0);
  await page.getByRole('link', { name: 'Reservas y página pública' }).first().click();
  await expect(page).toHaveURL(/s=enlinea/);
});
