import { apiAsClinic, apiJson, expect, test, uid } from '../support/app';

// An owner is one record: its pets are grouped under it, and two pets with the same name show whose each one is.
test('un propietario con varias mascotas, y dos mascotas con el mismo nombre', async ({ page }) => {
  const api = await apiAsClinic();
  const id = uid();
  const luis = `Luis${id} Pérez`;
  const marco = `Marco${id} Díaz`;
  const pet = async (names: string, owner: string, phone: string, extra: Record<string, unknown> = {}) =>
    apiJson(
      await api.post('/api/patients/', {
        data: {
          subject: 'animal', names, sex: 'Macho', birth_date: '2020-01-10', privacy_ack: true, guardian_name: owner, guardian_phone: phone,
          profile: { species: 'Perro', allergies_text: 'Ninguna conocida', sterilized: 'Sí' }, ...extra
        }
      })
    );
  const max1 = await pet(`Max${id}`, luis, '664 111 2222');
  await pet(`Luna${id}`, luis, '(664) 111-2222'); // the same person, typed differently
  await pet(`Max${id}`, marco, '664 333 4444'); // another Max, another owner
  await api.dispose();
  expect(max1.patient.owner_id).toBeTruthy();

  // by owner: searching the owner shows every pet under one card
  await page.goto('/pacientes');
  await page.getByRole('button', { name: 'Por propietario' }).click();
  await page.getByLabel('Buscar pacientes').fill(`Luis${id}`);
  const card = page.getByRole('listitem').filter({ hasText: luis }).first();
  await expect(card).toBeVisible();
  await expect(card.getByText(`Max${id}`)).toBeVisible();
  await expect(card.getByText(`Luna${id}`)).toBeVisible();
  await expect(card.getByText('2 mascotas')).toBeVisible();

  // searching the shared pet name shows each Max under its own owner
  await page.getByLabel('Buscar pacientes').fill(`Max${id}`);
  await expect(page.getByRole('listitem').filter({ hasText: luis }).first()).toBeVisible();
  await expect(page.getByRole('listitem').filter({ hasText: marco }).first()).toBeVisible();

  // by pet: one line per animal, each with its owner
  await page.getByRole('button', { name: 'Por mascota' }).click();
  await expect(page.getByText(`dueño: ${luis}`).first()).toBeVisible();
  await expect(page.getByText(`dueño: ${marco}`).first()).toBeVisible();

  // a new pet of an owner already registered: the owner comes preselected from the card
  await page.getByRole('button', { name: 'Por propietario' }).click();
  await page.getByLabel('Buscar pacientes').fill(`Luis${id}`);
  await page.getByRole('listitem').filter({ hasText: luis }).first().getByRole('link', { name: /Mascota/ }).click();
  await expect(page.getByRole('group', { name: 'Propietario elegido' })).toContainText(luis);
  await expect(page.getByRole('group', { name: 'Propietario elegido' })).toContainText(`Max${id}`);
});
