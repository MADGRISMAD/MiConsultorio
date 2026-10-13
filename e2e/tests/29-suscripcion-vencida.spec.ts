import { execFileSync } from 'node:child_process';
import { DATABASE_URL } from '../support/env.mjs';
import { expect, test } from '../support/app';

const sql = (q: string) => execFileSync('psql', [DATABASE_URL, '-v', 'ON_ERROR_STOP=1', '-qAt', '-c', q]).toString().trim();

// A clinic whose subscription lapsed: the administrator lands on the plans to pay, and the app does not let them in.
test('con la suscripción vencida el administrador llega a la ventana de planes y al pagar vuelve todo', async ({ page }) => {
  const before = sql(`SELECT billing_status || '|' || coalesce(trial_ends_at::text, '') || '|' || coalesce(current_period_end::text, '') FROM clinics ORDER BY created_at LIMIT 1`);
  try {
    sql(`UPDATE clinics SET billing_status = 'trialing', trial_ends_at = now() - interval '1 day', current_period_end = NULL`);
    await page.goto('/pacientes');
    await expect(page).toHaveURL(/\/suscripcion\?vencida=1/);
    await expect(page.getByText('Tus funciones están limitadas.')).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Suscripción y plan' })).toBeVisible();
  } finally {
    const [status, trial, period] = before.split('|');
    sql(`UPDATE clinics SET billing_status = '${status}', trial_ends_at = ${trial ? `'${trial}'` : 'NULL'}, current_period_end = ${period ? `'${period}'` : 'NULL'}`);
  }
  await page.goto('/pacientes');
  await expect(page).toHaveURL(/\/pacientes/);
  await expect(page.getByText('Tus funciones están limitadas.')).toHaveCount(0);
});
