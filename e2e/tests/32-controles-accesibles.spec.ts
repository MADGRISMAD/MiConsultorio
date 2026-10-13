import { apiAsClinic, createPersonApi, expect, test, uid } from '../support/app';

// The shared controls: the patient search is a real combobox (keyboard, active option announced) and the pill filters are
// groups of toggle buttons.
test('el buscador de pacientes se maneja con el teclado y anuncia la opción activa', async ({ page }) => {
  const api = await apiAsClinic();
  const last = `Teclado${uid()}`;
  const a = await createPersonApi(api, 'Ana', last);
  await createPersonApi(api, 'Beto', last);
  await api.dispose();

  await page.goto('/admin/admin-citas');
  await page.getByRole('button', { name: 'Nueva cita' }).first().click();
  const box = page.getByRole('combobox', { name: 'Paciente registrado' });
  await expect(box).toHaveAttribute('aria-expanded', 'false');
  await box.fill(last);
  const list = page.getByRole('listbox', { name: 'Paciente registrado' });
  // two matches plus «Registrar paciente nuevo»
  await expect(list.getByRole('option')).toHaveCount(3);
  await expect(box).toHaveAttribute('aria-expanded', 'true');
  // the first option is active; ArrowDown moves to the next one and the combobox points at it
  const first = await box.getAttribute('aria-activedescendant');
  await box.press('ArrowDown');
  const second = await box.getAttribute('aria-activedescendant');
  expect(second).not.toBe(first);
  await expect(page.locator(`#${second}`)).toHaveAttribute('aria-selected', 'true');
  // Escape closes the list, Enter picks
  await box.press('Escape');
  await expect(box).toHaveAttribute('aria-expanded', 'false');
  await box.press('ArrowDown');
  await box.press('Enter');
  await expect(page.getByText(/#\d+/).first()).toBeVisible();
  expect(a).toBeTruthy();
});

test('los filtros en píldora son botones con estado, no pestañas', async ({ page }) => {
  await page.goto('/avisos');
  const group = page.getByRole('group', { name: 'Filtro' });
  await expect(group.getByRole('button', { name: 'Todos' })).toHaveAttribute('aria-pressed', 'true');
  await group.getByRole('button', { name: /Sin leer/ }).click();
  await expect(group.getByRole('button', { name: /Sin leer/ })).toHaveAttribute('aria-pressed', 'true');
  await expect(group.getByRole('button', { name: 'Todos' })).toHaveAttribute('aria-pressed', 'false');
});
