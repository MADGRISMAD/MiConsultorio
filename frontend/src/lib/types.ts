export const PERMISSIONS = {
  adminUsers: 'adminUsers',
  adminAppointments: 'adminAppointments',
  adminHistorials: 'adminHistorials',
  navHistorials: 'navHistorials',
  navAppointments: 'navAppointments',
  pos: 'pos',
  posReports: 'posReports',
  posManage: 'posManage'
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
  /** Administrador de plataforma permanente: no se desactiva ni se le cambia el rol. */
  permanent?: boolean;
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
  cobros: boolean;
  magic_uses: number;
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
  admin: ['navAppointments', 'adminAppointments', 'navHistorials', 'adminHistorials', 'adminUsers', 'pos', 'posReports', 'posManage'],
  doctor: ['navAppointments', 'navHistorials', 'adminHistorials'],
  reception: ['navAppointments', 'adminAppointments', 'pos'],
  cashier: ['navAppointments', 'pos', 'posReports']
};

export const CAPABILITY_ROWS: [string, string][] = [
  ['Ver la agenda', 'navAppointments'],
  ['Crear y editar citas', 'adminAppointments'],
  ['Ver expedientes clínicos', 'navHistorials'],
  ['Crear y editar expedientes', 'adminHistorials'],
  ['Administrar el equipo', 'adminUsers'],
  ['Cobrar y manejar la caja (planes con cobros)', 'pos'],
  ['Ver reportes de ventas y facturas', 'posReports'],
  ['Editar catálogo, inventario y ajustes de cobros', 'posManage']
];

// ---------------------------------------------------------------------------
// Cobros (point of sale). Money is always in cents.
// ---------------------------------------------------------------------------

export type ItemKind = 'service' | 'product';

export interface CatalogItem {
  id: string;
  kind: ItemKind;
  name: string;
  sku: string;
  barcode: string;
  category: string;
  price_cents: number;
  cost_cents: number;
  tax_rate: number;
  track_stock: boolean;
  stock: number;
  min_stock: number;
  unit: string;
  active: boolean;
}

export type CatalogInput = Omit<CatalogItem, 'id' | 'active' | 'stock'> & { stock?: number; active?: boolean };

export type PayMethod = 'cash' | 'card' | 'transfer' | 'mp_point' | 'mp_link' | 'other';

export const PAY_METHODS: Record<PayMethod, { label: string; hint: string }> = {
  cash: { label: 'Efectivo', hint: 'Calcula el cambio' },
  card: { label: 'Tarjeta (terminal propia)', hint: 'Registra la referencia del voucher' },
  transfer: { label: 'Transferencia', hint: 'SPEI o depósito' },
  mp_point: { label: 'Terminal Mercado Pago Point', hint: 'Cobro en la terminal conectada' },
  mp_link: { label: 'Liga de pago Mercado Pago', hint: 'El paciente paga desde su celular' },
  other: { label: 'Otro', hint: 'Vales, cortesía pagada, etc.' }
};

export interface PrinterPrefs {
  kind: 'browser' | 'usb' | 'serial' | 'bluetooth';
  width: 58 | 80;
  copies: number;
  auto_print: boolean;
  open_drawer: boolean;
  cut: boolean;
}

export interface PosSettings {
  business_name: string;
  legal_name: string;
  rfc: string;
  tax_regime: string;
  tax_address: string;
  zip_code: string;
  phone: string;
  ticket_header: string;
  ticket_footer: string;
  show_tax_line: boolean;
  currency: 'MXN' | 'USD';
  default_tax_rate: number;
  allow_negative_stock: boolean;
  require_open_cash: boolean;
  allow_discounts: boolean;
  max_discount_pct: number;
  methods: PayMethod[];
  printer: PrinterPrefs;
}

export interface ProviderStatus {
  mp_configured: boolean;
  point_available: boolean;
  point_connected: boolean;
  point_account?: string;
  magic_available: boolean;
  mp_public_key?: string;
  sandbox: boolean;
}

export interface SaleLineInput {
  item_id?: string;
  name?: string;
  qty: number;
  unit_price_cents?: number;
  discount_cents?: number;
  tax_rate?: number;
}

export interface SalePaymentInput {
  method: PayMethod;
  amount_cents: number;
  received_cents?: number;
  reference?: string;
  intent_id?: string;
}

export interface SaleInput {
  lines: SaleLineInput[];
  discount_cents?: number;
  customer_name?: string;
  customer_curp?: string;
  appointment_id?: string;
  note?: string;
  payments: SalePaymentInput[];
}

export interface SaleLine {
  id: string;
  item_id: string | null;
  kind: ItemKind;
  name: string;
  qty: number;
  unit_price_cents: number;
  tax_rate: number;
  discount_cents: number;
  total_cents: number;
}

export interface SalePayment {
  method: PayMethod;
  amount_cents: number;
  received_cents: number | null;
  change_cents: number;
  reference: string;
}

export interface Sale {
  id: string;
  folio: number;
  customer_name: string;
  customer_curp: string;
  note: string;
  subtotal_cents: number;
  discount_cents: number;
  tax_cents: number;
  total_cents: number;
  status: 'paid' | 'void';
  void_reason: string;
  voided_by: string;
  created_by: string;
  created_at: string;
  lines?: SaleLine[];
  payments?: SalePayment[];
  invoice_status?: 'pending' | 'issued' | null;
}

export interface MethodTotal {
  method: PayMethod;
  amount_cents: number;
  count: number;
}

export interface CashMovement {
  id: string;
  kind: 'in' | 'out';
  amount_cents: number;
  concept: string;
  by: string;
  created_at: string;
}

export interface CashSession {
  id: string;
  opened_by: string;
  opened_at: string;
  opening_cents: number;
  closed_by: string;
  closed_at: string | null;
  counted_cents: number | null;
  expected_cents: number;
  diff_cents: number | null;
  note: string;
  sales: number;
  sales_cents: number;
  cash_in_cents: number;
  moves_in_cents: number;
  moves_out_cents: number;
  by_method: MethodTotal[];
  movements: CashMovement[] | null;
}

export interface PosReport {
  sales: number;
  void_sales: number;
  total_cents: number;
  tax_cents: number;
  discount_cents: number;
  avg_ticket_cents: number;
  cost_cents: number;
  margin_cents: number;
  by_method: MethodTotal[];
  by_day: { day: string; sales: number; total_cents: number }[];
  by_user: { name: string; sales: number; total_cents: number }[];
  top_items: { name: string; kind: ItemKind; qty: number; total_cents: number; margin_cents: number }[];
  inventory: { items: number; low_stock: number; value_cents: number; retail_cents: number };
}

export interface InvoiceRequest {
  id: string;
  sale_id: string;
  folio: number;
  total_cents: number;
  rfc: string;
  legal_name: string;
  tax_regime: string;
  zip_code: string;
  cfdi_use: string;
  email: string;
  status: 'pending' | 'issued' | 'cancelled';
  fiscal_uuid: string;
  note: string;
  created_by: string;
  created_at: string;
}

export interface StockMovement {
  id: string;
  delta: number;
  reason: 'initial' | 'purchase' | 'sale' | 'void' | 'adjustment' | 'loss';
  note: string;
  balance: number;
  by: string;
  created_at: string;
}

export interface PointDevice {
  id: string;
  operating_mode: string;
  pos_id: number;
  store_id: string;
}

export interface Charge {
  id: string;
  kind: 'point' | 'link';
  status: 'open' | 'approved' | 'canceled' | 'error';
  amount_cents: number;
  pay_url?: string;
  used: boolean;
}

export interface MagicItem {
  name: string;
  kind: ItemKind;
  category: string;
  price_cents: number;
  cost_cents: number;
  stock: number;
  unit: string;
  barcode: string;
}

export interface PlanOffer extends Plan {
  month_cents: number;
  year_cents: number;
  online: boolean;
}

export interface CheckoutRow {
  id: string;
  plan: string;
  period: 'month' | 'year';
  amount_cents: number;
  status: 'pending' | 'paid' | 'failed' | 'expired';
  init_point: string;
  created_at: string;
  paid_at: string | null;
}
