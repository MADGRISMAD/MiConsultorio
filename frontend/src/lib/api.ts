import type {
  ActivityItem,
  Appointment,
  AppointmentInput,
  AccessEntry,
  CashSession,
  CatalogInput,
  CatalogItem,
  Charge,
  CheckoutRow,
  ComplianceItem,
  Clinic,
  ClinicRow,
  ClinicSettings,
  Encounter,
  EncounterInput,
  InvoiceRequest,
  Legal,
  MagicItem,
  Overview,
  Patient,
  PatientInput,
  PatientRecord,
  PatientRow,
  PatientSchema,
  Payment,
  Person,
  Plan,
  PlanOffer,
  PointState,
  PointTerminal,
  PosReport,
  PosSettings,
  Prescription,
  PrescriptionInput,
  QuickPatientInput,
  Issuer,
  ProviderStatus,
  Sale,
  SaleInput,
  Seats,
  SessionInfo,
  StockMovement,
  Billing
} from './types';

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    /** machine-readable reason sent by the server (CASH_CLOSED, NO_STOCK, PLAN_REQUIRED, ...) */
    readonly code = ''
  ) {
    super(message);
  }
}

export async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  let res: Response;
  try {
    res = await fetch(`/api${path}`, {
      method,
      credentials: 'same-origin',
      headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body)
    });
  } catch {
    throw new ApiError('No se pudo conectar con el servidor.', 0);
  }
  if (res.status === 204) return undefined as T;
  const data = await res.json().catch(() => null);
  if (!res.ok) throw new ApiError(data?.message ?? 'Ocurrió un error inesperado.', res.status, data?.code ?? '');
  return data as T;
}

export const seg = encodeURIComponent;

export const api = {
  login: (identifier: string, password: string) =>
    request<{ session: SessionInfo }>('POST', '/login', { identifier, password }).then((r) => r.session),
  register: (r: { clinic_name: string; phone: string; name: string; email: string; username: string; password: string }) =>
    request<{ session: SessionInfo }>('POST', '/register', r).then((x) => x.session),
  logout: () => request<void>('POST', '/logout'),
  forgotPassword: (identifier: string) => request<{ ok: boolean }>('POST', '/forgot', { identifier }),
  resetPassword: (token: string, password: string) => request<{ ok: boolean }>('POST', '/reset-password', { token, password }),
  session: () => request<{ session: SessionInfo }>('GET', '/session').then((r) => r.session),
  clinic: () => request<{ clinic: Clinic }>('GET', '/clinic').then((r) => r.clinic),
  updateClinic: (patch: Partial<{ name: string; phone_number: string; address: string; kind: string; specialties: string[]; settings: ClinicSettings }>) =>
    request<{ clinic: Clinic }>('PUT', '/clinic', patch).then((r) => r.clinic),
  completeSetup: () => request<{ session: SessionInfo }>('POST', '/clinic/setup').then((r) => r.session),

  // my account
  updateProfile: (
    name: string,
    phone: string,
    pro?: { cedula: string; cedula_institution: string; cedula_specialty: string; specialty_title: string }
  ) => request<{ session: SessionInfo }>('PUT', '/me', { name, phone, ...pro }).then((r) => r.session),
  changePassword: (current_password: string, new_password: string) =>
    request<{ session: SessionInfo }>('PUT', '/me/password', { current_password, new_password }).then((r) => r.session),

  // clinic team
  team: () => request<{ people: Person[]; seats: Seats; roles: { id: string; label: string }[] }>('GET', '/team/'),
  addMember: (m: { name: string; email: string; username: string; phone: string; password: string; role: string }) =>
    request<{ person: Person }>('POST', '/team/', m),
  updateMember: (id: string, patch: Partial<{ name: string; email: string; phone: string; role: string; permissions_extra: string[]; permissions_denied: string[] }>) =>
    request<{ person: Person }>('PATCH', `/team/${seg(id)}`, patch),
  resetMemberPassword: (id: string, password: string) => request<void>('POST', `/team/${seg(id)}/password`, { password }),
  deactivateMember: (id: string) => request<void>('POST', `/team/${seg(id)}/deactivate`),
  reactivateMember: (id: string) => request<void>('POST', `/team/${seg(id)}/reactivate`),

  appointments: () => request<{ appointments: Appointment[] }>('GET', '/appointments/').then((r) => r.appointments),
  appointment: (id: string) => request<{ appointment: Appointment }>('GET', `/appointments/${seg(id)}`).then((r) => r.appointment),
  createAppointment: (a: AppointmentInput) => request<unknown>('POST', '/appointments/', a),
  updateAppointment: (id: string, a: AppointmentInput) => request<unknown>('PUT', `/appointments/${seg(id)}`, a),
  deleteAppointment: (id: string) => request<void>('DELETE', `/appointments/${seg(id)}`),

  // pacientes (personas y animales), bitácora y recetas
  patients: {
    schema: () => request<PatientSchema>('GET', '/patients/schema'),
    list: (p: { q?: string; archived?: boolean; pending?: boolean } = {}) => {
      const q = new URLSearchParams();
      if (p.q) q.set('q', p.q);
      if (p.archived) q.set('archived', '1');
      if (p.pending) q.set('pending', '1');
      return request<{ patients: PatientRow[] }>('GET', `/patients/?${q}`).then((r) => r.patients);
    },
    /** name and contact only: what the front desk needs to book a visit */
    lookup: (q: string) => request<{ patients: PatientRow[] }>('GET', `/patients/lookup?q=${encodeURIComponent(q)}`).then((r) => r.patients),
    get: (id: string) => request<{ patient: Patient }>('GET', `/patients/${seg(id)}`).then((r) => r.patient),
    create: (p: PatientInput) => request<{ patient: Patient }>('POST', '/patients/', p).then((r) => r.patient),
    quick: (p: QuickPatientInput) => request<{ patient: Patient }>('POST', '/patients/quick', p).then((r) => r.patient),
    update: (id: string, p: PatientInput) => request<{ patient: Patient }>('PUT', `/patients/${seg(id)}`, p).then((r) => r.patient),
    privacy: (id: string) => request<{ patient: Patient }>('POST', `/patients/${seg(id)}/privacy`).then((r) => r.patient),
    archive: (id: string, reason: string) => request<{ patient: Patient }>('POST', `/patients/${seg(id)}/archive`, { reason }).then((r) => r.patient),
    unarchive: (id: string) => request<{ patient: Patient }>('POST', `/patients/${seg(id)}/unarchive`).then((r) => r.patient),
    access: (id: string) => request<{ access: AccessEntry[] }>('GET', `/patients/${seg(id)}/access`).then((r) => r.access),
    encounters: (id: string) => request<{ encounters: Encounter[] }>('GET', `/patients/${seg(id)}/encounters`).then((r) => r.encounters),
    createEncounter: (id: string, e: EncounterInput) => request<{ encounter: Encounter }>('POST', `/patients/${seg(id)}/encounters`, e).then((r) => r.encounter),
    addendum: (encounterId: string, a: { reason: string; text: string }) =>
      request<{ encounter: Encounter }>('POST', `/encounters/${seg(encounterId)}/addendum`, a).then((r) => r.encounter),
    prescriptions: (id: string) => request<{ prescriptions: Prescription[] }>('GET', `/patients/${seg(id)}/prescriptions`).then((r) => r.prescriptions),
    createPrescription: (id: string, p: PrescriptionInput) => request<{ prescription: Prescription }>('POST', `/patients/${seg(id)}/prescriptions`, p).then((r) => r.prescription),
    /** everything for printing the expediente (logs the access) */
    record: (id: string) => request<PatientRecord>('GET', `/patients/${seg(id)}/record`)
  },
  prescriptions: {
    /** receta + patient + establishment, ready to print (logs the access) */
    print: (id: string) => request<{ prescription: Prescription; patient: Patient; clinic: Issuer }>('GET', `/prescriptions/${seg(id)}`),
    void: (id: string, reason: string) => request<unknown>('POST', `/prescriptions/${seg(id)}/void`, { reason })
  },
  legal: {
    get: () => request<{ legal: Legal }>('GET', '/clinic/legal').then((r) => r.legal),
    save: (l: Legal) => request<{ legal: Legal }>('PUT', '/clinic/legal', l).then((r) => r.legal),
    compliance: () => request<{ items: ComplianceItem[] }>('GET', '/clinic/compliance').then((r) => r.items)
  },

  // billing: paying for the plan online
  billing: {
    overview: () =>
      request<{ offers: PlanOffer[]; billing: Billing; checkouts: CheckoutRow[]; online: boolean; sandbox: boolean; currency: string }>('GET', '/billing/'),
    checkout: (plan: string, period: 'month' | 'year') => request<{ id: string; init_point: string }>('POST', '/billing/checkout', { plan, period }),
    checkoutStatus: (id: string) => request<{ checkout: CheckoutRow }>('GET', `/billing/checkouts/${seg(id)}`).then((r) => r.checkout)
  },

  // cobros (point of sale)
  pos: {
    settings: () => request<{ settings: PosSettings; providers: ProviderStatus }>('GET', '/pos/settings'),
    saveSettings: (s: PosSettings) => request<{ settings: PosSettings }>('PUT', '/pos/settings', s).then((r) => r.settings),

    items: (p: { kind?: string; q?: string; active?: boolean; low?: boolean; category?: string } = {}) => {
      const q = new URLSearchParams();
      if (p.kind) q.set('kind', p.kind);
      if (p.q) q.set('q', p.q);
      if (p.active) q.set('active', '1');
      if (p.low) q.set('low', '1');
      if (p.category) q.set('category', p.category);
      return request<{ items: CatalogItem[]; categories: string[] }>('GET', `/pos/items?${q}`);
    },
    createItem: (i: CatalogInput) => request<{ item: CatalogItem }>('POST', '/pos/items', i).then((r) => r.item),
    updateItem: (id: string, i: CatalogInput) => request<{ item: CatalogItem }>('PUT', `/pos/items/${seg(id)}`, i).then((r) => r.item),
    deleteItem: (id: string) => request<{ archived: boolean }>('DELETE', `/pos/items/${seg(id)}`),
    importItems: (items: CatalogInput[]) => request<{ created: number; skipped: string[] }>('POST', '/pos/items/import', { items }),
    adjustStock: (id: string, a: { delta?: number; set_to?: number; reason: 'purchase' | 'adjustment' | 'loss'; note?: string; lot_code?: string; expires_on?: string; lot_id?: string }) =>
      request<{ stock: number }>('POST', `/pos/items/${seg(id)}/stock`, a),
    stockMovements: (id: string) => request<{ movements: StockMovement[] }>('GET', `/pos/items/${seg(id)}/movements`).then((r) => r.movements),

    createSale: (s: SaleInput) => request<{ sale: Sale }>('POST', '/pos/sales', s).then((r) => r.sale),
    sales: (p: { from?: string; to?: string; q?: string; status?: string; invoiceable?: boolean; limit?: number } = {}) => {
      const q = new URLSearchParams();
      for (const [k, v] of Object.entries(p)) if (v !== undefined && v !== '' && v !== false) q.set(k, v === true ? '1' : String(v));
      return request<{ sales: Sale[] }>('GET', `/pos/sales?${q}`).then((r) => r.sales);
    },
    sale: (id: string) => request<{ sale: Sale }>('GET', `/pos/sales/${seg(id)}`).then((r) => r.sale),
    emailSale: (id: string, email: string) => request<{ ok: boolean }>('POST', `/pos/sales/${seg(id)}/email`, { email }),
    voidSale: (id: string, reason: string) => request<unknown>('POST', `/pos/sales/${seg(id)}/void`, { reason }),

    cash: () => request<{ session: CashSession | null }>('GET', '/pos/cash/current').then((r) => r.session),
    openCash: (opening_cents: number) => request<{ session: CashSession }>('POST', '/pos/cash/open', { opening_cents }).then((r) => r.session),
    cashMove: (m: { kind: 'in' | 'out'; amount_cents: number; concept: string }) => request<unknown>('POST', '/pos/cash/movements', m),
    closeCash: (counted_cents: number, note: string) => request<{ session: CashSession }>('POST', '/pos/cash/close', { counted_cents, note }).then((r) => r.session),
    cashSessions: () => request<{ sessions: CashSession[] }>('GET', '/pos/cash/sessions').then((r) => r.sessions),
    cashSession: (id: string) => request<{ session: CashSession }>('GET', `/pos/cash/sessions/${seg(id)}`).then((r) => r.session),

    report: (from = '', to = '') => request<{ report: PosReport; from: string; to: string }>('GET', `/pos/reports?from=${seg(from)}&to=${seg(to)}`),
    salesCsvUrl: (from = '', to = '') => `/api/pos/reports/sales.csv?from=${seg(from)}&to=${seg(to)}`,

    invoices: (status = '') => request<{ invoices: InvoiceRequest[] }>('GET', `/pos/invoices?status=${seg(status)}`).then((r) => r.invoices),
    requestInvoice: (i: { sale_id: string; rfc: string; legal_name: string; tax_regime: string; zip_code: string; cfdi_use: string; email: string }) =>
      request<{ id: string }>('POST', '/pos/invoices', i),
    updateInvoice: (id: string, p: { status: 'issued' | 'cancelled'; fiscal_uuid?: string; note?: string }) => request<unknown>('PATCH', `/pos/invoices/${seg(id)}`, p),

    // Mercado Pago
    pointConnectUrl: () => request<{ url: string }>('GET', '/pos/point/connect').then((r) => r.url),
    pointDisconnect: () => request<unknown>('POST', '/pos/point/disconnect'),
    pointStatus: () => request<PointState>('GET', '/pos/point/status'),
    pointTerminals: () => request<{ terminals: PointTerminal[] }>('GET', '/pos/point/terminals').then((r) => r.terminals),
    pointRegister: (terminal_id: string) => request<{ terminal_id: string; label: string }>('POST', '/pos/point/terminal', { terminal_id }),
    pointCharge: (amount_cents: number) => request<{ id: string }>('POST', '/pos/point/charges', { amount_cents }),
    payLink: (amount_cents: number, title: string) => request<{ id: string; url: string }>('POST', '/pos/mp/links', { amount_cents, title }),
    charge: (id: string) => request<{ charge: Charge }>('GET', `/pos/charges/${seg(id)}`).then((r) => r.charge),
    cancelCharge: (id: string) => request<unknown>('DELETE', `/pos/charges/${seg(id)}`),

    // magic (AI)
    magic: () => request<{ available: boolean; limit: number; used: number }>('GET', '/pos/magic'),
    magicInventory: (p: { text?: string; image_base64?: string; mime?: string }) => request<{ items: MagicItem[] }>('POST', '/pos/magic/inventory', p).then((r) => r.items),
    magicPrice: (margin_pct: number, items: { id: string; name: string; cost_cents: number }[]) =>
      request<{ suggestions: { id: string; price_cents: number; note: string }[]; ai: boolean }>('POST', '/pos/magic/price', { margin_pct, items })
  },

  // platform
  platform: {
    overview: () => request<Overview>('GET', '/platform/overview'),
    plans: () => request<{ plans: Plan[] }>('GET', '/platform/plans').then((r) => r.plans),
    clinics: (q = '', state = '') =>
      request<{ clinics: ClinicRow[]; counts: Record<string, number> }>('GET', `/platform/clinics?q=${encodeURIComponent(q)}&state=${encodeURIComponent(state)}`),
    clinic: (id: string) =>
      request<{ clinic: ClinicRow; people: Person[]; seats: Seats; activity: ActivityItem[]; payments: Payment[] }>('GET', `/platform/clinics/${seg(id)}`),
    updateClinic: (id: string, patch: Record<string, unknown>) => request<{ clinic: ClinicRow }>('PATCH', `/platform/clinics/${seg(id)}`, patch).then((r) => r.clinic),
    suspend: (id: string, reason: string) => request<{ clinic: ClinicRow }>('POST', `/platform/clinics/${seg(id)}/suspend`, { reason }).then((r) => r.clinic),
    reactivate: (id: string) => request<{ clinic: ClinicRow }>('POST', `/platform/clinics/${seg(id)}/reactivate`).then((r) => r.clinic),
    pay: (id: string, p: { amount: number; months: number; note: string }) => request<{ period_end: string }>('POST', `/platform/clinics/${seg(id)}/payments`, p),
    activity: (clinicId = '') => request<{ activity: ActivityItem[] }>('GET', `/platform/activity?limit=200${clinicId ? `&clinic_id=${seg(clinicId)}` : ''}`).then((r) => r.activity),
    staff: () => request<{ people: Person[]; roles: { id: string; label: string }[] }>('GET', '/platform/staff'),
    addStaff: (m: { name: string; email: string; username: string; password: string; role: string }) => request<{ person: Person }>('POST', '/platform/staff', m),
    updateStaff: (id: string, patch: Partial<{ name: string; role: string }>) => request<{ person: Person }>('PATCH', `/platform/staff/${seg(id)}`, patch),
    resetStaffPassword: (id: string, password: string) => request<void>('POST', `/platform/staff/${seg(id)}/password`, { password }),
    deactivateStaff: (id: string) => request<void>('POST', `/platform/staff/${seg(id)}/deactivate`),
    reactivateStaff: (id: string) => request<void>('POST', `/platform/staff/${seg(id)}/reactivate`)
  }
};
