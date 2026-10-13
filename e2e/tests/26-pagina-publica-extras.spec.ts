import { apiAsClinic, apiJson, expect, test, uid } from '../support/app';

// La página pública: botón flotante de WhatsApp, servicios destacados, motivos, seguros, preguntas y redes.
test('la página pública muestra WhatsApp flotante, destacados, motivos, seguros, preguntas y redes', async ({ page }) => {
  const api = await apiAsClinic();
  const slug = `extras-${uid()}`.toLowerCase();
  const st = (await apiJson(await api.get('/api/agenda/settings'))).settings;
  await apiJson(await api.put('/api/agenda/settings', { data: { ...st, booking_enabled: true, booking_slug: slug } }));
  const prof = await apiJson(await api.get('/api/clinic/profile'));
  const original = prof.profile;
  const svc = prof.services[0];
  await apiJson(
    await api.put('/api/clinic/profile', {
      data: {
        ...original,
        enabled: true,
        tagline: 'Cuidamos tu salud',
        whatsapp: '5215512345678',
        accepts_insurance: true,
        insurances: ['GNP Seguros', 'AXA', 'Una aseguradora local'],
        highlights: ['Equipo especializado', 'Ambiente moderno'],
        faqs: [{ q: '¿Atienden niños?', a: 'Sí, atendemos a todas las edades.' }],
        instagram_url: '@clinicaextras',
        facebook_url: 'clinicaextras',
        public_services: [svc.id],
        service_details: [{ id: svc.id, description: 'Una cita para salir con diagnóstico y plan', featured: true }]
      }
    })
  );
  try {
    await page.goto(`/${slug}`);
    await expect(page.getByRole('link', { name: /Escribir por WhatsApp/ })).toHaveAttribute('href', /wa\.me\/5215512345678/);

    await expect(page.getByRole('heading', { name: 'Empieza por aquí' })).toBeVisible();
    await expect(page.getByText('Una cita para salir con diagnóstico y plan')).toBeVisible();

    await expect(page.getByRole('list', { name: 'Por qué elegirnos' }).getByText('Ambiente moderno')).toBeVisible();

    await expect(page.getByRole('heading', { name: 'Aceptamos seguros' })).toBeVisible();
    await expect(page.getByText('GNP Seguros')).toBeVisible();
    await expect(page.getByText('Una aseguradora local')).toBeVisible();

    const q = page.getByText('¿Atienden niños?');
    await q.click();
    await expect(page.getByText('Sí, atendemos a todas las edades.')).toBeVisible();

    await expect(page.getByRole('link', { name: /Instagram de/ })).toHaveAttribute('href', 'https://www.instagram.com/clinicaextras');
    await expect(page.getByRole('link', { name: /Facebook de/ })).toHaveAttribute('href', 'https://www.facebook.com/clinicaextras');

    // sin aceptar seguros la sección desaparece
    await apiJson(await api.put('/api/clinic/profile', { data: { ...original, enabled: true, whatsapp: '5215512345678', accepts_insurance: false, insurances: [] } }));
    await page.reload();
    await expect(page.getByRole('heading', { name: 'Aceptamos seguros' })).toHaveCount(0);
  } finally {
    await api.put('/api/clinic/profile', { data: { ...original, service_details: [{ id: svc.id, description: '', featured: false }] } });
    await api.put('/api/agenda/settings', { data: st });
    await api.dispose();
  }
});

// El editor: seguros con casillas por aseguradora, ejemplos por giro y servicios destacados.
test('el editor de la página ofrece aseguradoras, ejemplos por giro y destacados', async ({ page }) => {
  await page.goto('/ajustes?s=enlinea&t=pagina');
  await expect(page.getByText('Por qué elegirnos').first()).toBeVisible();
  await page.getByRole('button', { name: 'Usar ejemplos de mi giro' }).first().click();
  await expect(page.getByLabel('Motivo 1', { exact: true })).not.toHaveValue('');
  await expect(page.getByLabel('Pregunta 1', { exact: true })).not.toHaveValue('');
  await expect(page.getByRole('heading', { name: 'Preguntas frecuentes' }).or(page.getByText('Preguntas frecuentes').first())).toBeVisible();

  await page.goto('/ajustes?s=enlinea&t=directorio');
  const acepto = page.getByRole('checkbox', { name: /Acepto seguros/ });
  await acepto.check();
  await expect(page.getByText('GNP Seguros')).toBeVisible();
  await expect(page.getByText('Seguros Monterrey New York Life')).toBeVisible();
});
