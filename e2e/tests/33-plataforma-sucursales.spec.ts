import { ANON_STATE, PLATFORM, apiAnon, apiJson, apiLogin, expect, login, test, uid } from '../support/app';

test.use({ storageState: ANON_STATE });

// The branches are not clients of their own: the general list shows the matrix and how many it has, and the matrix profile lists them.
test('plataforma: las sucursales no salen en la lista general; la matriz las lista y muestra cuántas tiene', async ({ page }) => {
  const id = uid();
  const owner = { username: `dueno${id}`, password: 'clave-segura-1', clinic: `Matriz ${id}` };
  const platform = await apiLogin(PLATFORM.username, PLATFORM.password);

  // a clinic of its own with the Pro plan (the branches come with it)
  const api = await apiAnon();
  const session = (await apiJson(await api.post('/api/register', { data: { clinic_name: owner.clinic, phone: '5511111111', name: `Dueño ${id}`, email: `${owner.username}@e2e.test`, username: owner.username, password: owner.password, accepted_terms: true } }))).session;
  await apiJson(await platform.patch(`/api/platform/clinics/${session.clinicId}`, { data: { plan: 'pro', billing_status: 'active' } }));

  const branchName = `Sucursal Norte ${id}`;
  const branch = (await apiJson(await api.post('/api/org/branches', { data: { name: branchName, kind: 'DENTAL', phone_number: '5511112222', address: 'Calle 1' } }))).branch;
  await apiJson(await api.post(`/api/org/branches/${branch.id}/suspend`));

  // the general list shows the matrix with its number of branches, and not the branches themselves
  await login(page, PLATFORM.username, PLATFORM.password);
  await page.goto('/plataforma/negocios');
  await page.getByRole('searchbox', { name: 'Buscar negocio' }).fill(owner.clinic);
  const matrix = page.getByRole('button', { name: new RegExp(`^${owner.clinic}`) });
  await expect(matrix).toContainText('Matriz · 1 sucursal');
  await expect(page.getByRole('button', { name: new RegExp(branchName) })).toHaveCount(0);

  // inside the matrix, the Sucursales tab lists it as suspended
  await matrix.click();
  await page.getByRole('tab', { name: /Sucursales \(1\)/ }).click();
  const listed = page.getByRole('list', { name: new RegExp(`Sucursales de ${owner.clinic}`) });
  await expect(listed).toContainText(branchName);
  await expect(listed).toContainText('Sucursal suspendida');

  // a branch has no platform Suspend/Reactivate: it is managed from the matrix
  await listed.getByRole('link', { name: new RegExp(branchName) }).click();
  await expect(page.getByRole('heading', { name: branchName })).toBeVisible();
  await expect(page.getByRole('button', { name: /^Suspender$|^Reactivar$/ })).toHaveCount(0);
  await expect(page.getByText('Sucursal de').first()).toBeVisible();

  await api.dispose();
  await platform.dispose();
});
