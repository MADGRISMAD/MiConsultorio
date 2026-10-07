import { expect, test } from '../support/app';

// Installable app: manifest, service worker, offline page and the rule that clinical data (/api) is never cached.

test('el manifest cumple lo necesario para instalar la aplicación', async ({ page, request }) => {
  await page.goto('/login');
  await expect(page.locator('link[rel="manifest"]')).toHaveAttribute('href', /manifest\.webmanifest$/);

  const res = await request.get('/manifest.webmanifest');
  expect(res.status()).toBe(200);
  expect(res.headers()['content-type']).toContain('application/manifest+json');
  expect(res.headers()['cache-control']).toBe('no-cache');
  const m = await res.json();
  expect(m.name).toBe('Caresia');
  expect(m.display).toBe('standalone');
  expect(m.start_url).toBe('/');
  expect(m.theme_color).toBe('#0B2540');
  const sizes = m.icons.map((i: { sizes: string; purpose: string }) => `${i.sizes}:${i.purpose}`);
  expect(sizes).toEqual(expect.arrayContaining(['192x192:any', '512x512:any', '512x512:maskable']));
  for (const icon of m.icons) {
    const r = await request.get(icon.src);
    expect(r.status()).toBe(200);
    expect(r.headers()['content-type']).toBe('image/png');
  }

  const sw = await request.get('/service-worker.js');
  expect(sw.status()).toBe(200);
  expect(sw.headers()['cache-control']).toBe('no-cache');
  expect(sw.headers()['service-worker-allowed']).toBe('/');
});

test('service worker: precachea el shell, /offline sin red y /api nunca se guarda ni se sirve de copia', async ({ page, context }) => {
  await page.goto('/pacientes');
  await page.evaluate(() => navigator.serviceWorker.ready);
  // the first load is not controlled yet; after a reload the worker answers
  await page.reload();
  await expect.poll(() => page.evaluate(() => !!navigator.serviceWorker.controller)).toBe(true);

  const cached = await page.evaluate(async () => {
    const out: string[] = [];
    for (const key of await caches.keys()) for (const req of await (await caches.open(key)).keys()) out.push(new URL(req.url).pathname);
    return { keys: await caches.keys(), paths: out };
  });
  expect(cached.keys.some((k) => k.startsWith('caresia-static-'))).toBe(true);
  expect(cached.paths).toContain('/offline');
  expect(cached.paths.some((p) => p.startsWith('/_app/immutable/'))).toBe(true);

  // a clinical request while online works and is not stored
  const online = await page.evaluate(async () => (await fetch('/api/patients/')).status);
  expect(online).toBe(200);
  const after = await page.evaluate(async () => {
    const out: string[] = [];
    for (const key of await caches.keys()) for (const req of await (await caches.open(key)).keys()) out.push(new URL(req.url).pathname);
    return out;
  });
  expect(after.some((p) => p.startsWith('/api'))).toBe(false);

  // without network the API fails instead of answering from a copy
  await context.setOffline(true);
  const failed = await page.evaluate(() => fetch('/api/patients/').then(() => 'served', () => 'failed'));
  expect(failed).toBe('failed');

  // and a navigation shows the offline page
  await page.goto('/agenda').catch(() => {});
  await expect(page.getByRole('heading', { name: 'Sin conexión' })).toBeVisible();
  expect(new URL(page.url()).pathname).toBe('/offline');
  expect(new URL(page.url()).searchParams.get('from')).toBe('/agenda');
  await context.setOffline(false);

  // back online, "Reintentar" returns to the page they wanted
  await page.getByRole('link', { name: 'Reintentar' }).click();
  await page.waitForURL('**/agenda');
});
