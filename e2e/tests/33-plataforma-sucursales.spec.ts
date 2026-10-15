import { ANON_STATE, PLATFORM, apiAnon, apiJson, apiLogin, expect, login, test, uid } from '../support/app';

test.use({ storageState: ANON_STATE });

// The platform panel shows a branch the owner switched off, and lists the branches inside their matrix.
test('plataforma: la sucursal suspendida por su dueño se ve como tal y las sucursales cuelgan de su matriz', async ({ page }) => {
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

  // the platform administrator sees it as suspended, under its matrix
  await login(page, PLATFORM.username, PLATFORM.password);
  await page.goto('/plataforma/negocios');
  await page.getByRole('searchbox', { name: 'Buscar negocio' }).fill(id);
  const matrix = page.getByRole('button', { name: new RegExp(`^Matriz ${id}`) });
  await expect(matrix).toContainText('1 sucursal');
  const row = page.getByRole('button', { name: new RegExp(branchName) });
  await expect(row).toContainText(`Sucursal de Matriz ${id}`);
  await expect(row).toContainText('Sucursal suspendida');

  // its own filter
  await page.getByRole('searchbox', { name: 'Buscar negocio' }).fill('');
  await page.getByRole('button', { name: /^Sucursal suspendida/ }).click();
  await expect(page.getByRole('button', { name: new RegExp(branchName) })).toBeVisible();

  // inside the matrix, the Sucursales tab lists it
  await page.getByRole('button', { name: /^Todos/ }).click();
  await page.getByRole('searchbox', { name: 'Buscar negocio' }).fill(`Matriz ${id}`);
  await matrix.click();
  await page.getByRole('tab', { name: /Sucursales \(1\)/ }).click();
  const listed = page.getByRole('list', { name: new RegExp(`Sucursales de Matriz ${id}`) });
  await expect(listed).toContainText(branchName);
  await expect(listed).toContainText('Sucursal suspendida');

  // a branch has no platform Suspend/Reactivate: it is managed from the matrix
  await listed.getByRole('link', { name: new RegExp(branchName) }).click();
  await expect(page.getByRole('heading', { name: branchName })).toBeVisible();
  await expect(page.getByRole('button', { name: /^Suspender$|^Reactivar$/ })).toHaveCount(0);
  await expect(page.getByText(`Sucursal de`).first()).toBeVisible();

  await api.dispose();
  await platform.dispose();
});
