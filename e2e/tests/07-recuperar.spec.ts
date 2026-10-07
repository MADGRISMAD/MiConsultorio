import { BASE, expect, mailbox, test, ui, uid, waitForMail } from '../support/app';

test.use({ storageState: { cookies: [], origins: [] } });

// Uses its own account (created through the API) so the sample clinic's password never changes.
test('recuperación de contraseña con correo', async ({ page, request }) => {
  const id = uid();
  const user = `rec${id}`;
  const email = `${user}@e2e.test`;
  const reg = await request.post('/api/register', {
    data: { clinic_name: `Rec ${id}`, phone: '', name: 'Persona Recupera', email, username: user, password: 'clave-original-1' }
  });
  expect(reg.ok()).toBeTruthy();
  mailbox.clear();

  await page.goto('/forgot');
  await page.fill('#fg-id', user);
  await page.click(ui.submit);
  await expect(page.getByText('Revisa tu correo')).toBeVisible();

  const link = await waitForMail(new RegExp(`${BASE.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}/restablecer\\?token=[A-Za-z0-9_%-]+`));
  await page.goto(link);
  await page.fill('#rp-pass', 'nueva-clave-larga-9');
  await page.fill('#rp-confirm', 'nueva-clave-larga-9');
  await page.click(ui.submit);
  await expect(page.getByText('Contraseña actualizada')).toBeVisible();

  // the link works once
  await page.goto(link);
  await page.fill('#rp-pass', 'otra-clave-larga-9');
  await page.fill('#rp-confirm', 'otra-clave-larga-9');
  await page.click(ui.submit);
  await expect(page.getByRole('alert').first()).toBeVisible();

  // and the new password is the one that works
  await page.goto('/login');
  await page.fill(ui.loginId, user);
  await page.fill(ui.loginPass, 'nueva-clave-larga-9');
  await page.click(ui.submit);
  await page.waitForURL((u) => !u.pathname.startsWith('/login'));
});
