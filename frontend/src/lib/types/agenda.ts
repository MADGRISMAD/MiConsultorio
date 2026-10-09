export type ApptStatus = 'scheduled' | 'confirmed' | 'arrived' | 'in_progress' | 'completed' | 'no_show' | 'cancelled';

export interface Appt {
  id: string;
  patient_id: string | null;
  names: string;
  last_names: string;
  CURP: string;
  date: string;
  startHour: string;
  endHour: string;
  details: string;
  professional_id: string | null;
  professional_name: string;
  status: ApptStatus;
  room: string;
  source: 'staff' | 'online' | 'portal';
  service_id: string | null;
  service_name: string;
  phone: string;
  email: string;
  encounter_id: string | null;
  sale_id: string | null;
  cancel_reason: string;
  arrived_at: string | null;
  started_at: string | null;
  finished_at: string | null;
  reminders_consent: boolean;
  /** the appointment's date and time (clinic zone) as an instant, and when it can start being worked (one hour before) */
  starts_at: string | null;
  opens_at: string | null;
  /** present only on the answer that closed the appointment: the consultation was queued at the register */
  charge?: { id: string; total_cents: number };
}

export interface ApptInput {
  patient_id: string | null;
  names: string;
  last_names: string;
  CURP: string;
  date: string;
  startHour: string;
  endHour: string;
  details: string;
  professional_id: string | null;
  service_id: string | null;
  room: string;
  phone: string;
  email: string;
  reminders_consent: boolean;
  overbook: boolean;
}

export function emptyApptInput(): ApptInput {
  return {
    patient_id: null, names: '', last_names: '', CURP: '', date: '', startHour: '', endHour: '', details: '',
    professional_id: null, service_id: null, room: '', phone: '', email: '', reminders_consent: true, overbook: false
  };
}

export function inputFromAppt(a: Appt): ApptInput {
  return {
    patient_id: a.patient_id, names: a.names, last_names: a.last_names, CURP: a.CURP, date: a.date, startHour: a.startHour, endHour: a.endHour,
    details: a.details, professional_id: a.professional_id, service_id: a.service_id, room: a.room, phone: a.phone, email: a.email, reminders_consent: a.reminders_consent, overbook: false
  };
}

export type DayKey = 'mon' | 'tue' | 'wed' | 'thu' | 'fri' | 'sat' | 'sun';
export type HoursMap = Partial<Record<DayKey, [string, string][]>>;

export interface Professional {
  id: string;
  name: string;
  role: string;
  specialty: string;
  bookable: boolean;
  /** sees patients; false for an owner who only runs the clinic */
  consults: boolean;
  slot_minutes: number;
  hours: HoursMap;
  color: string;
}

export interface ProfessionalInput {
  bookable: boolean;
  consults?: boolean;
  slot_minutes: number;
  hours: HoursMap;
  color: string;
}

export interface TimeBlock {
  id: string;
  professional_id: string | null;
  date_from: string;
  date_to: string;
  startHour: string | null;
  endHour: string | null;
  reason: string;
  created_by_name: string;
}

export interface TimeBlockInput {
  professional_id: string | null;
  date_from: string;
  date_to: string;
  startHour: string;
  endHour: string;
  reason: string;
}

export interface AgendaSettings {
  slot_minutes: number;
  rooms: string[];
  booking_enabled: boolean;
  booking_slug: string;
  booking_lead_hours: number;
  booking_horizon_days: number;
  booking_message: string;
  booking_requires_confirmation: boolean;
  booking_show_prices: boolean;
  cancel_min_hours: number;
  remind_email: boolean;
  remind_whatsapp: boolean;
  remind_hours: number[];
  reminder_template: string;
}

export interface ServiceOption {
  id: string;
  name: string;
}

export interface ApptFilters {
  from?: string;
  to?: string;
  professional?: string;
  status?: string;
  room?: string;
  patient?: string;
}

/** Label and colour classes (Tailwind, theme tokens) of each status. */
export const STATUS_META: Record<ApptStatus, { label: string; tone: 'info' | 'ok' | 'warn' | 'bad' | 'muted'; card: string }> = {
  scheduled: { label: 'Por confirmar', tone: 'info', card: 'bg-app-primary/14 text-app-ink border-app-primary' },
  confirmed: { label: 'Confirmada', tone: 'ok', card: 'bg-app-accent/16 text-app-ink border-app-accent' },
  arrived: { label: 'Llegó', tone: 'warn', card: 'bg-app-warning/16 text-app-ink border-app-warning' },
  in_progress: { label: 'En consulta', tone: 'info', card: 'bg-app-primary/30 text-app-ink border-app-primary' },
  completed: { label: 'Completada', tone: 'muted', card: 'bg-app-ink/8 text-app-muted border-app-ink/30' },
  no_show: { label: 'No asistió', tone: 'bad', card: 'bg-app-danger/10 text-app-muted border-app-danger/60' },
  cancelled: { label: 'Cancelada', tone: 'bad', card: 'bg-app-danger/8 text-app-muted border-app-danger/40 line-through decoration-app-danger/50' }
};

export const OPEN_STATUSES: ApptStatus[] = ['scheduled', 'confirmed', 'arrived', 'in_progress'];
/** Appointments that still hold their slot. */
export const isActive = (a: Appt) => a.status !== 'cancelled' && a.status !== 'no_show';

export const PRO_COLORS = ['#1673d1', '#0f9e8e', '#c2410c', '#7c3aed', '#be185d', '#4d7c0f', '#0e7490', '#a16207'];
