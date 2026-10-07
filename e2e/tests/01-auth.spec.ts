import { CLINIC, expect, login, logout, test, ui, uid } from '../support/app';

test.use({ storageState: { cookies: [], origins: [] } });

test('un consultorio nuevo se registra, cierra sesión y vuelve a entrar', async ({ page }) => {
  const id = uid();
  const user = `reg${id}`;
  const pass = 'clave-registro-1';

  await page.goto('/register');
  await page.fill(ui.register.clinic, `Clínica ${id}`);
  await page.fill(ui.register.phone, '5512345678');
  await page.fill(ui.register.person, 'Persona Registro');
  await page.fill(ui.register.email, `${user}@e2e.test`);
  await page.fill(ui.register.user, user);
  await page.fill(ui.register.pass, pass);
  await page.fill(ui.register.confirm, pass);
  await page.getByRole('checkbox').check();
  await page.getByRole('button', { name: 'Crear mi consultorio' }).click();
  // a new clinic goes through the welcome wizard before anything else
  await page.waitForURL('**/bienvenida');
  await expect(page.getByRole('heading', { name: /Bienvenido/ })).toBeVisible();

  await page.getByRole('button', { name: 'Configurar después' }).click(); // skip the wizard
  await page.waitForURL((u) => u.pathname === '/');
  await logout(page);
  await login(page, user, pass);
  await expect(page).not.toHaveURL(/\/login/);
});

test('el acceso rechaza contraseñas incorrectas y protege las pantallas', async ({ page }) => {
  await page.goto('/pacientes');
  await page.waitForURL('**/login');

  await page.fill(ui.loginId, CLINIC.username);
  await page.fill(ui.loginPass, 'contraseña-equivocada');
  await page.click(ui.submit);
  await expect(page.getByRole('alert').first()).toBeVisible();
  await expect(page).toHaveURL(/\/login/);

  await page.fill(ui.loginPass, CLINIC.password);
  await page.click(ui.submit);
  await page.waitForURL((u) => !u.pathname.startsWith('/login'));
  await page.reload();
  await expect(page.getByRole('button', { name: 'Cerrar sesión' }).first()).toBeVisible(); // the session survives a reload
});
