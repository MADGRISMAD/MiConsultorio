import { apiAsClinic, apiJson, clinicNow, createPersonApi, expect, hhmm, test, uid } from '../support/app';

// Quien cobra vincula al paciente desde la cita de la agenda (nombre y contacto), sin abrir expedientes.
test('cobrar una cita de hoy desde el punto de venta trae al paciente vinculado', async ({ page }) => {
  const id = uid();
  const names = `Cita${id}`;
  const api = await apiAsClinic();
  const patient = await createPersonApi(api, names, 'Cobro');
  const pros = (await apiJson(await api.get('/api/agenda/professionals'))).professionals as { id: string; consults: boolean }[];
  const { date, minutes } = clinicNow();
  const start = Math.min(Math.max(0, minutes - 30), 23 * 60);
  const appt = (
    await apiJson(
      await api.post('/api/appointments/', {
        data: {
          patient_id: patient, names, last_names: 'Cobro', CURP: '', date, startHour: hhmm(start), endHour: hhmm(start + 20), details: 'Revisión',
          professional_id: (pros.find((p) => p.consults) ?? pros[0]).id, service_id: null, room: '', phone: '', email: '', reminders_consent: false, overbook: true
        }
      })
    )
  ).appointment;
  await apiJson(await api.post(`/api/appointments/${appt.id}/status`, { data: { status: 'arrived', reason: '', keep_deposit: false } }));
  await api.dispose();

  await page.goto('/pos/cobros');
  const panel = page.getByTestId('today-appointments-panel');
  await expect(panel.getByText(`${names} Cobro`)).toBeVisible();
  await panel.getByRole('button', { name: new RegExp(`Cobrar la cita de ${names}`) }).click();

  // el paciente queda vinculado desde la cita
  await expect(page.getByTestId('cart-origin')).toContainText(`${names} Cobro`);
  await expect(page.getByText('Paciente vinculado a su expediente.')).toBeVisible();
});
