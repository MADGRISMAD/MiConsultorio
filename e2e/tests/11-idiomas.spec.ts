import { expect, test } from '../support/app';

// Public pages follow the visitor: Spanish by default, English by browser language or by the selector.
test.use({ storageState: { cookies: [], origins: [] } });

test('español por defecto, selector de idioma y <html lang>', async ({ page }) => {
  await page.goto('/verificar/token-que-no-existe');
  await expect(page.getByRole('heading', { name: 'No encontramos esta receta' })).toBeVisible();
  await expect(page.locator('html')).toHaveAttribute('lang', 'es');

  const group = page.getByRole('group', { name: 'Idioma' });
  await group.getByRole('button', { name: 'English' }).click();
  await expect(page.getByRole('heading', { name: 'We could not find this prescription' })).toBeVisible();
  await expect(page.locator('html')).toHaveAttribute('lang', 'en');
  expect(await page.evaluate(() => localStorage.getItem('caresia_ui_lang'))).toBe('en');

  // the choice survives a reload and applies to the other public pages
  await page.goto('/cita/token-que-no-existe');
  await expect(page.getByRole('heading', { name: 'We could not find that appointment' })).toBeVisible();
  await page.getByRole('group', { name: 'Language' }).getByRole('button', { name: 'Español' }).click();
  await expect(page.getByRole('heading', { name: 'No encontramos esa cita' })).toBeVisible();
});

test.describe('navegador en inglés', () => {
  test.use({ locale: 'en-US' });

  test('las páginas públicas se muestran en inglés sin elegir', async ({ page }) => {
    await page.goto('/arco/clinica-inexistente');
    await expect(page.getByRole('heading', { name: 'This page is not available' })).toBeVisible();
    await expect(page.locator('html')).toHaveAttribute('lang', 'en');
  });

  test('la app de los empleados sigue en español', async ({ page }) => {
    await page.goto('/login');
    await expect(page.locator('html')).toHaveAttribute('lang', 'es');
    await expect(page.getByRole('button', { name: /Entrar|Iniciar sesión/ }).first()).toBeVisible();
  });

  test('sin conexión: la página offline sin ruta de origen vuelve al inicio', async ({ page }) => {
    await page.goto('/offline?from=//evil.example');
    await expect(page.getByRole('heading', { name: 'You are offline' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Try again' })).toHaveAttribute('href', '/');
  });
});
