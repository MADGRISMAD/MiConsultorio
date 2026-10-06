export const PERMISSIONS = {
  adminUsers: 'adminUsers',
  adminAppointments: 'adminAppointments',
  adminHistorials: 'adminHistorials',
  navHistorials: 'navHistorials',
  navAppointments: 'navAppointments'
} as const;

export type Permission = (typeof PERMISSIONS)[keyof typeof PERMISSIONS];

export type ClinicRole = 'admin' | 'doctor' | 'reception' | 'cashier';
export type PlatformRole = 'platform_admin' | 'platform_support';
export type Role = ClinicRole | PlatformRole;

/** What each role does, in the words shown to people (the server enforces the real rules). */
export const ROLES: Record<Role, { label: string; short: string; about: string; tone: 'info' | 'ok' | 'warn' | 'muted' }> = {
  admin: { label: 'Administrador', short: 'Admin', about: 'Todo, incluido el equipo y sus roles.', tone: 'info' },
  doctor: { label: 'Médico / especialista', short: 'Médico', about: 'Ve la agenda y lee y edita los expedientes clínicos.', tone: 'ok' },
  reception: { label: 'Recepción', short: 'Recepción', about: 'Ve y administra la agenda. No ve información clínica.', tone: 'warn' },
  cashier: { label: 'Cajero', short: 'Cajero', about: 'Ve la agenda. Los cobros llegarán con el punto de venta.', tone: 'muted' },
  platform_admin: { label: 'Administrador de plataforma', short: 'Admin', about: 'Negocios, suscripciones, pagos y equipo de plataforma.', tone: 'info' },
  platform_support: { label: 'Soporte', short: 'Soporte', about: 'Consulta negocios y su actividad. No cambia nada.', tone: 'muted' }
};

export const CLINIC_ROLES: ClinicRole[] = ['admin', 'doctor', 'reception', 'cashier'];
export const PLATFORM_ROLES: PlatformRole[] = ['platform_admin', 'platform_support'];

export type BillingState = 'trialing' | 'trial_expired' | 'active' | 'past_due' | 'suspended';

export const BILLING_STATES: Record<BillingState, { label: string; tone: 'ok' | 'info' | 'warn' | 'bad' }> = {
  trialing: { label: 'En prueba', tone: 'info' },
  trial_expired: { label: 'Prueba vencida', tone: 'warn' },
  active: { label: 'Activo', tone: 'ok' },
  past_due: { label: 'Pago atrasado', tone: 'warn' },
  suspended: { label: 'Suspendido', tone: 'bad' }
};

export interface Billing {
  plan: string;
  plan_name: string;
  state: BillingState;
  usable: boolean;
  trial_ends_at: string | null;
  trial_days_left: number | null;
  current_period_end: string | null;
  suspended_reason: string;
  /** the plan includes the collections (cobros) section */
  cobros: boolean;
}

export interface SessionInfo {
  userId: string;
  clinicId: string;
  username: string;
  name: string;
  email: string;
  role: Role;
  roleLabel: string;
  permissions: string[];
  /** null for platform staff */
  billing: Billing | null;
  /** a clinic administrator who still has to finish the setup wizard */
  setupPending: boolean;
}

export interface Person {
  id: string;
  name: string;
  email: string;
  username: string;
  phone: string;
  role: Role;
  role_label: string;
  disabled: boolean;
  last_login_at: string | null;
  created_at: string;
}

export interface Seats {
  plan: string;
  plan_name: string;
  max_users: number | null;
  max_doctors: number | null;
  used_users: number;
  used_doctors: number;
}

export interface Plan {
  id: string;
  name: string;
  price_month: number;
  max_users: number | null;
  max_doctors: number | null;
  description: string;
}

export interface ClinicRow {
  id: string;
  name: string;
  kind: ClinicKind;
  email: string;
  phone_number: string;
  address: string;
  plan: string;
  plan_name: string;
  billing_status: string;
  state: BillingState;
  trial_ends_at: string | null;
  trial_days_left: number | null;
  current_period_end: string | null;
  suspended_reason: string;
  created_at: string;
  users: number;
  owner_name: string;
  owner_email: string;
  last_seen: string | null;
}

export interface ActivityItem {
  id: number;
  clinic_id: string | null;
  clinic_name: string;
  actor_name: string;
  type: string;
  message: string;
  created_at: string;
}

export interface Payment {
  id: string;
  amount_cents: number;
  plan: string;
  months: number;
  note: string;
  period_end: string;
  created_by: string;
  created_at: string;
}

export interface AttentionItem {
  kind: 'past_due' | 'trial_expired' | 'trial_ending';
  clinic_id: string;
  clinic_name: string;
  title: string;
  detail: string;
  since: string | null;
}

export interface Overview {
  counts: Record<string, number>;
  attention: AttentionItem[];
  recent: ClinicRow[];
  signups_30d: number;
  /** administrators only */
  mrr?: number;
  paid_this_month_cents?: number;
}

import type { IconName } from './components/ui/Icon.svelte';

/** Business types ("giro"). Keys match the server; order is the order shown in the wizard. */
export const CLINIC_KINDS = {
  GENERAL_MEDICAL: { label: 'Medicina general', hint: 'Consultorio médico', icon: 'stethoscope' },
  DENTAL: { label: 'Odontología', hint: 'Clínica dental', icon: 'tooth' },
  PEDIATRICS: { label: 'Pediatría', hint: 'Niñas, niños y adolescentes', icon: 'baby' },
  INTERNAL_MEDICINE: { label: 'Medicina interna', hint: 'Adultos y padecimientos crónicos', icon: 'heart' },
  PHYSIOTHERAPY: { label: 'Fisioterapia', hint: 'Rehabilitación y terapia física', icon: 'activity' },
  NUTRITION: { label: 'Nutrición', hint: 'Planes y seguimiento nutricional', icon: 'leaf' },
  PSYCHOLOGY: { label: 'Psicología', hint: 'Salud mental y terapia', icon: 'chat' },
  DERMATOLOGY: { label: 'Dermatología', hint: 'Piel, cabello y uñas', icon: 'droplet' },
  GYNECOLOGY: { label: 'Ginecología', hint: 'Salud de la mujer', icon: 'flower' },
  ORTHOPEDICS: { label: 'Ortopedia', hint: 'Huesos, músculos y articulaciones', icon: 'bone' },
  VETERINARY: { label: 'Veterinaria', hint: 'Clínica veterinaria', icon: 'paw' },
  CHIROPRACTIC: { label: 'Quiropráctica', hint: 'Quiropráctica y columna', icon: 'spine' }
} as const satisfies Record<string, { label: string; hint: string; icon: IconName }>;

export type ClinicKind = keyof typeof CLINIC_KINDS;
export const CLINIC_KIND_KEYS = Object.keys(CLINIC_KINDS) as ClinicKind[];

export const WEEKDAYS = [
  ['mon', 'Lunes'],
  ['tue', 'Martes'],
  ['wed', 'Miércoles'],
  ['thu', 'Jueves'],
  ['fri', 'Viernes'],
  ['sat', 'Sábado'],
  ['sun', 'Domingo']
] as const;
export type Weekday = (typeof WEEKDAYS)[number][0];

export interface DayHours {
  open: boolean;
  start: string;
  end: string;
}

export interface ClinicSettings {
  hours: Record<Weekday, DayHours>;
  appointment_minutes: number;
}

export interface Clinic {
  id: string;
  kind: ClinicKind;
  /** other specialties besides the main kind */
  specialties: ClinicKind[];
  name: string;
  phone_number: string;
  address: string;
  image_url: string;
  settings: ClinicSettings;
  setup_completed: boolean;
}

export interface User {
  username: string;
  permissions: string[];
}

export const CHECKBOX_FIELDS = [
  'diabetes',
  'rheumatic_diseases',
  'fractures',
  'allergies',
  'layed',
  'contractures',
  'cancer',
  'accidents',
  'transfusions',
  'cardiopathies',
  'surgeries',
  'tabaquism',
  'alcoholism',
  'automedication',
  'drug_use',
  'pregnant'
] as const;

export type CheckboxField = (typeof CHECKBOX_FIELDS)[number];

export interface ExpedientInput extends Record<CheckboxField, boolean> {
  CURP: string;
  names: string;
  last_names: string;
  sex: 'Hombre' | 'Mujer';
  date_of_birth: string;
  education: string;
  occupation: string;
  weight: string;
  clothes_size: string;
  height: string;
  ethnicity: string;
  physical_activity: string;
  hobbies: string;
  child: string;
}

export interface Expedient extends ExpedientInput {
  id: string;
  age: number;
}

export interface AppointmentInput {
  names: string;
  last_names: string;
  CURP: string;
  date: string;
  startHour: string;
  endHour: string;
  details: string;
}

export interface Appointment extends AppointmentInput {
  id: string;
}

export function emptyExpedient(): ExpedientInput {
  return {
    CURP: '',
    names: '',
    last_names: '',
    sex: 'Hombre',
    date_of_birth: '',
    education: '',
    occupation: '',
    weight: '',
    clothes_size: '',
    height: '',
    ethnicity: '',
    physical_activity: '',
    hobbies: '',
    child: '',
    ...(Object.fromEntries(CHECKBOX_FIELDS.map((f) => [f, false])) as Record<CheckboxField, boolean>)
  };
}

export function emptyAppointment(): AppointmentInput {
  return { names: '', last_names: '', CURP: '', date: '', startHour: '', endHour: '', details: '' };
}

/** What each clinic role can do, for the "who can do what" table (the server decides for real). */
export const ROLE_PERMISSIONS: Record<ClinicRole, string[]> = {
  admin: ['navAppointments', 'adminAppointments', 'navHistorials', 'adminHistorials', 'adminUsers'],
  doctor: ['navAppointments', 'navHistorials', 'adminHistorials'],
  reception: ['navAppointments', 'adminAppointments'],
  cashier: ['navAppointments']
};

export const CAPABILITY_ROWS: [string, string][] = [
  ['Ver la agenda', 'navAppointments'],
  ['Crear y editar citas', 'adminAppointments'],
  ['Ver expedientes clínicos', 'navHistorials'],
  ['Crear y editar expedientes', 'adminHistorials'],
  ['Administrar el equipo', 'adminUsers']
];
