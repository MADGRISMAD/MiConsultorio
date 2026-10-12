import { agendaApi } from '$lib/api/agenda';
import type { Appt } from '$lib/types/agenda';

/** El expediente del paciente con la nota de consulta ya abierta y ligada a la cita (al guardarla, la cita se cierra y pasa a caja). */
export const consultUrl = (a: Appt) => `/pacientes/${encodeURIComponent(a.patient_id ?? '')}?cita=${encodeURIComponent(a.id)}&motivo=${encodeURIComponent(a.details || a.service_name)}`;

/** Marca la llegada si hace falta y pasa la cita a «en consulta». Devuelve la cita ya actualizada. */
export async function beginConsult(a: Appt): Promise<Appt> {
  let cur = a;
  if (cur.status === 'scheduled' || cur.status === 'confirmed') cur = await agendaApi.setStatus(cur.id, 'arrived');
  if (cur.status === 'arrived') cur = await agendaApi.setStatus(cur.id, 'in_progress');
  return cur;
}
