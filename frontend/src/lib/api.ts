import type {
  ActivityItem,
  Appointment,
  AppointmentInput,
  CashSession,
  CatalogInput,
  CatalogItem,
  Charge,
  CheckoutRow,
  Clinic,
  ClinicRow,
  ClinicSettings,
  Expedient,
  ExpedientInput,
  InvoiceRequest,
  MagicItem,
  Overview,
  Payment,
  Person,
  Plan,
  PlanOffer,
  PointDevice,
  PosReport,
  PosSettings,
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

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
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

const seg = encodeURIComponent;

export const api = {
  login: (identifier: string, password: string) =>
    request<{ session: SessionInfo }>('POST', '/login', { identifier, password }).then((r) => r.session),
  register: (r: { clinic_name: string; phone: string; name: string; email: string; username: string; password: string }) =>
    request<{ session: SessionInfo }>('POST', '/register', r).then((x) => x.session),
  logout: () => request<void>('POST', '/logout'),
  session: () => request<{ session: SessionInfo }>('GET', '/session').then((r) => r.session),
  clinic: () => request<{ clinic: Clinic }>('GET', '/clinic').then((r) => r.clinic),
  updateClinic: (patch: Partial<{ name: string; phone_number: string; address: string; kind: string; specialties: string[]; settings: ClinicSettings }>) =>
    request<{ clinic: Clinic }>('PUT', '/clinic', patch).then((r) => r.clinic),
  completeSetup: () => request<{ session: SessionInfo }>('POST', '/clinic/setup').then((r) => r.session),

  // my account
  updateProfile: (name: string, phone: string) => request<{ session: SessionInfo }>('PUT', '/me', { name, phone }).then((r) => r.session),
  changePassword: (current_password: string, new_password: string) =>
    request<{ session: SessionInfo }>('PUT', '/me/password', { current_password, new_password }).then((r) => r.session),

  // clinic team
  team: () => request<{ people: Person[]; seats: Seats; roles: { id: string; label: string }[] }>('GET', '/team/'),
  addMember: (m: { name: string; email: string; username: string; phone: string; password: string; role: string }) =>
    request<{ person: Person }>('POST', '/team/', m),
  updateMember: (id: string, patch: Partial<{ name: string; email: string; phone: string; role: string }>) =>
    request<{ person: Person }>('PATCH', `/team/${seg(id)}`, patch),
  resetMemberPassword: (id: string, password: string) => request<void>('POST', `/team/${seg(id)}/password`, { password }),
  deactivateMember: (id: string) => request<void>('POST', `/team/${seg(id)}/deactivate`),
  reactivateMember: (id: string) => request<void>('POST', `/team/${seg(id)}/reactivate`),

  expedients: () => request<{ expedients: Expedient[] }>('GET', '/expedients/').then((r) => r.expedients),
  expedient: (curp: string) => request<{ expedient: Expedient }>('GET', `/expedients/${seg(curp)}`).then((r) => r.expedient),
  createExpedient: (e: ExpedientInput) => request<unknown>('POST', '/expedients/', e),
  updateExpedient: (curp: string, e: ExpedientInput) => request<unknown>('PUT', `/expedients/${seg(curp)}`, e),
  deleteExpedient: (curp: string) => request<void>('DELETE', `/expedients/${seg(curp)}`),

  appointments: () => request<{ appointments: Appointment[] }>('GET', '/appointments/').then((r) => r.appointments),
  appointment: (id: string) => request<{ appointment: Appointment }>('GET', `/appointments/${seg(id)}`).then((r) => r.appointment),
  createAppointment: (a: AppointmentInput) => request<unknown>('POST', '/appointments/', a),
  updateAppointment: (id: string, a: AppointmentInput) => request<unknown>('PUT', `/appointments/${seg(id)}`, a),
  deleteAppointment: (id: string) => request<void>('DELETE', `/appointments/${seg(id)}`),

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
    adjustStock: (id: string, a: { delta?: number; set_to?: number; reason: 'purchase' | 'adjustment' | 'loss'; note?: string }) =>
      request<{ stock: number }>('POST', `/pos/items/${seg(id)}/stock`, a),
    stockMovements: (id: string) => request<{ movements: StockMovement[] }>('GET', `/pos/items/${seg(id)}/movements`).then((r) => r.movements),

    createSale: (s: SaleInput) => request<{ sale: Sale }>('POST', '/pos/sales', s).then((r) => r.sale),
    sales: (p: { from?: string; to?: string; q?: string; status?: string; invoiceable?: boolean; limit?: number } = {}) => {
      const q = new URLSearchParams();
      for (const [k, v] of Object.entries(p)) if (v !== undefined && v !== '' && v !== false) q.set(k, v === true ? '1' : String(v));
      return request<{ sales: Sale[] }>('GET', `/pos/sales?${q}`).then((r) => r.sales);
    },
    sale: (id: string) => request<{ sale: Sale }>('GET', `/pos/sales/${seg(id)}`).then((r) => r.sale),
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
    pointDevices: () => request<{ devices: PointDevice[] }>('GET', '/pos/point/devices').then((r) => r.devices),
    pointMode: (id: string, mode: 'PDV' | 'STANDALONE') => request<unknown>('PATCH', `/pos/point/devices/${seg(id)}`, { mode }),
    pointCharge: (device_id: string, amount_cents: number) => request<{ id: string }>('POST', '/pos/point/intents', { device_id, amount_cents }),
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
