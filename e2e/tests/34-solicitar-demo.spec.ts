import { ANON_STATE, apiJson, apiLogin, expect, PLATFORM, test, ui, uid, waitForMail } from '../support/app';

test.use({ storageState: ANON_STATE });

// The "Solicitar demo" card says which plan the person wants to try: every account starts on Básico and the team is told
// which plan to activate.
test('cada tarjeta de plan lleva al registro con su plan; la cuenta empieza en Básico y el equipo recibe el plan pedido', async ({ page }) => {
  // the cards (and the button next to "Entrar") lead to the sign-up
  await page.goto('/');
  await expect(page.getByRole('link', { name: 'Probar gratis' }).first()).toHaveAttribute('href', '/register');
  for (const plan of ['basico', 'crecimiento', 'pro']) {
    await expect(page.locator(`a[href="/register?plan=${plan}"]`)).toBeAttached({ timeout: 15_000 });
  }

  // with the plan of the card: the notice on the form
  await page.locator('a[href="/register?plan=crecimiento"]').first().click();
  await page.waitForURL('**/register?plan=crecimiento');
  await expect(page.getByRole('status').filter({ hasText: 'Crecimiento' })).toContainText('Crea tu cuenta para continuar');

  const id = uid();
  const user = `demo${id}`;
  await page.fill(ui.register.clinic, `Demo ${id}`);
  await page.fill(ui.register.phone, '5512345678');
  await page.fill(ui.register.person, 'Persona Demo');
  await page.fill(ui.register.email, `${user}@e2e.test`);
  await page.fill(ui.register.user, user);
  await page.fill(ui.register.pass, 'clave-registro-1');
  await page.fill(ui.register.confirm, 'clave-registro-1');
  await page.getByRole('checkbox').check();
  await page.getByRole('button', { name: 'Crear mi consultorio' }).click();
  await page.waitForURL('**/bienvenida');

  // the team gets the plan asked for, and the account is Básico on trial
  await waitForMail(new RegExp(`Plan solicitado: Crecimiento[\\s\\S]{0,200}Demo ${id}|Demo ${id}[\\s\\S]{0,400}Plan solicitado: Crecimiento`));
  const platform = await apiLogin(PLATFORM.username, PLATFORM.password);
  const list = (await apiJson(await platform.get(`/api/platform/clinics?q=${encodeURIComponent(`Demo ${id}`)}`))).clinics as { plan: string; state: string; requested_plan: string }[];
  expect(list).toHaveLength(1);
  expect(list[0]).toMatchObject({ plan: 'basico', state: 'trialing', requested_plan: 'crecimiento' });
  await platform.dispose();
});

test('sin plan (Probar gratis) no hay aviso en el registro; y un plan inventado se ignora', async ({ page }) => {
  await page.goto('/register');
  await expect(page.getByText('Tu solicitud de demo')).toHaveCount(0);
  await page.goto('/register?plan=constructor');
  await expect(page.getByText('Tu solicitud de demo')).toHaveCount(0);
  await page.goto('/demo'); // the old demo link leads to the sign-up
  await page.waitForURL('**/register');
});
