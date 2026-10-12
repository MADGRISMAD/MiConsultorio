import { apiAsClinic, apiJson, createPersonApi, expect, test, ui, uid } from '../support/app';

// Va al final de la suite a propósito: al completar la cita deja una pre-cuenta en caja, y las pruebas de cobros esperan caja limpia.
// El consultorio de ejemplo trabaja en hora de Ciudad de México: el navegador usa la misma zona para que «hoy» coincida.
const TZ = 'America/Mexico_City';
test.use({ timezoneId: TZ });

/** Fecha y hora actuales en la zona del consultorio. */
function clinicNow() {
  const p = Object.fromEntries(
    new Intl.DateTimeFormat('en-CA', { timeZone: TZ, year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hourCycle: 'h23' })
      .formatToParts(new Date())
      .map((x) => [x.type, x.value])
  );
  return { date: `${p.year}-${p.month}-${p.day}`, minutes: Number(p.hour) * 60 + Number(p.minute) };
}
const hhmm = (m: number) => `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`;

test('desde el inicio: iniciar la consulta de una cita abre la nota ligada y al guardarla la cita se completa', async ({ page }) => {
  const id = uid();
  const names = `Inicio${id}`;
  const api = await apiAsClinic();
  const patient = await createPersonApi(api, names, 'Consulta');
  const pros = (await apiJson(await api.get('/api/agenda/professionals'))).professionals as { id: string; consults: boolean }[];
  const { date, minutes } = clinicNow();
  const start = Math.min(Math.max(0, minutes - 30), 23 * 60); // ya es su hora, a cualquier hora del día
  await apiJson(
    await api.post('/api/appointments/', {
      data: {
        patient_id: patient, names, last_names: 'Consulta', CURP: '', date, startHour: hhmm(start), endHour: hhmm(start + 20), details: 'Revisión general',
        professional_id: (pros.find((p) => p.consults) ?? pros[0]).id, service_id: null, room: '', phone: '', email: '', reminders_consent: false, overbook: true
      }
    })
  );
  await api.dispose();

  await page.goto('/');
  const row = page.getByRole('listitem').filter({ hasText: names });
  await expect(row).toBeVisible();
  await expect(row.getByText('Por confirmar')).toBeVisible();

  // un clic: marca llegada, pasa a consulta y abre el expediente con la nota lista
  await row.getByRole('button', { name: 'Iniciar consulta' }).click();
  await page.waitForURL(new RegExp(`/pacientes/${patient}`));
  await expect(page.locator(ui.enc.reason)).toHaveValue('Revisión general');
  await page.fill(ui.enc.subjective, `Viene a revisión ${id}.`);
  await page.fill(ui.enc.assessment, 'Sin hallazgos');
  await page.getByRole('button', { name: 'Guardar en la bitácora' }).click();
  await expect(page.getByText(`Viene a revisión ${id}.`)).toBeVisible();

  // la cita quedó atendida
  await page.goto('/');
  await expect(page.getByRole('listitem').filter({ hasText: names }).getByText('Completada')).toBeVisible();
});
