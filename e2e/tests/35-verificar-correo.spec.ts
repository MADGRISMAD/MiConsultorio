import { ANON_STATE, expect, test, ui, uid, verificationCode } from '../support/app';

test.use({ storageState: ANON_STATE });

// The account is only created with the code that arrives in the mailbox: stops made-up addresses and mass sign-ups.
test('el registro pide el código del correo: uno equivocado no crea la cuenta y el correcto sí', async ({ page }) => {
  const id = uid();
  const user = `ver${id}`;
  const email = `${user}@e2e.test`;
  await page.goto('/register');
  await page.fill(ui.register.clinic, `Verificada ${id}`);
  await page.fill(ui.register.person, 'Persona Verifica');
  await page.fill(ui.register.email, email);
  await page.fill(ui.register.user, user);
  await page.fill(ui.register.pass, 'clave-registro-1');
  await page.fill(ui.register.confirm, 'clave-registro-1');
  await page.getByRole('checkbox').check();

  // step 1: the code is mailed and asked for on the same form
  await page.getByRole('button', { name: 'Crear mi consultorio' }).click();
  await expect(page.getByLabel('Código de verificación')).toBeVisible();
  await expect(page.getByText(email)).toBeVisible();
  await expect(page.getByRole('button', { name: /Reenviar en \d+ s/ })).toBeDisabled();
  const code = await verificationCode(email);

  // without the code, or with a wrong one, nothing is created
  await page.getByRole('button', { name: 'Verificar y crear mi consultorio' }).click();
  await expect(page.getByText('Escribe el código de 6 dígitos')).toBeVisible();
  await page.fill('#r-code', code === '000000' ? '111111' : '000000');
  await page.getByRole('button', { name: 'Verificar y crear mi consultorio' }).click();
  await expect(page.getByText('El código no es correcto.')).toBeVisible();
  await expect(page).toHaveURL(/\/register/);

  // the right one creates the account
  await page.fill('#r-code', code);
  await page.getByRole('button', { name: 'Verificar y crear mi consultorio' }).click();
  await page.waitForURL('**/bienvenida');
});

test('si cambia el correo, hay que pedir otro código', async ({ page }) => {
  const id = uid();
  await page.goto('/register');
  await page.fill(ui.register.clinic, `Cambia ${id}`);
  await page.fill(ui.register.person, 'Persona Cambia');
  await page.fill(ui.register.email, `cambia${id}@e2e.test`);
  await page.fill(ui.register.user, `cambia${id}`);
  await page.fill(ui.register.pass, 'clave-registro-1');
  await page.fill(ui.register.confirm, 'clave-registro-1');
  await page.getByRole('checkbox').check();
  await page.getByRole('button', { name: 'Crear mi consultorio' }).click();
  await expect(page.getByLabel('Código de verificación')).toBeVisible();
  await page.fill(ui.register.email, `otro${id}@e2e.test`);
  await expect(page.getByLabel('Código de verificación')).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Crear mi consultorio' })).toBeVisible();
});
