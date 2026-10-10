import { dateTime } from '$lib/format';
import type { ArcoDeadlineState, ArcoKind, ArcoRequest, ArcoStatus } from '$lib/types/arco';

export const KIND_LABEL: Record<ArcoKind, string> = {
  acceso: 'Acceso',
  rectificacion: 'Rectificación',
  cancelacion: 'Cancelación',
  oposicion: 'Oposición',
  revocacion: 'Revocación del consentimiento'
};

export const KINDS = Object.keys(KIND_LABEL) as ArcoKind[];

export const STATUS_LABEL: Record<ArcoStatus, string> = {
  recibida: 'Recibida',
  en_revision: 'En revisión',
  requiere_info: 'Requiere información',
  atendida: 'Atendida',
  negada: 'Negada',
  vencida: 'Vencida'
};

export const statusPill = (s: ArcoStatus) =>
  s === 'atendida' ? 'pill-ok' : s === 'negada' ? 'pill' : s === 'requiere_info' ? 'pill-warn' : s === 'vencida' ? 'pill-bad' : 'pill-info';

export const deadlinePill = (s: ArcoDeadlineState) => (s === 'overdue' ? 'pill-bad' : s === 'soon' ? 'pill-warn' : 'pill');

/** Texto corto del plazo pendiente: «Vence en 5 días hábiles», «Vencida hace 2 días hábiles». */
export function deadlineText(r: ArcoRequest): string {
  if (r.days_left === null || !r.deadline) return '';
  const what = r.deadline === 'execute' ? 'Ejecutar' : 'Responder';
  const n = Math.abs(r.days_left);
  const days = `${n} día${n === 1 ? '' : 's'} hábil${n === 1 ? '' : 'es'}`;
  if (r.days_left < 0) return `${what}: vencida hace ${days}`;
  if (r.days_left === 0) return `${what}: vence hoy`;
  return `${what}: ${days} restantes`;
}

export const LEGAL_REMINDER = 'Los plazos se cuentan en días hábiles (lunes a viernes, sin festivos). Son una guía: verifica los plazos con tu asesor legal.';

export function fmtDay(d: string | null | undefined): string {
  if (!d) return '—';
  const [y, m, day] = d.slice(0, 10).split('-').map(Number);
  return new Date(y, m - 1, day).toLocaleDateString('es-MX', { day: 'numeric', month: 'short', year: 'numeric' });
}

export function fmtDateTime(iso: string | null | undefined): string {
  if (!iso) return '—';
  return dateTime(iso);
}

/** A request that was fully carried out: answered and, when it was granted, executed. */
export const arcoDone = (r: ArcoRequest) => r.status === 'atendida' && !r.pending_execute;
