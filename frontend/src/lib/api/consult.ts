import { request, seg } from '$lib/api';
import type { Charge, ChargeCatalogItem, ChargeInput, ChargeStatus, ComplementResult, PaymentComplements } from '$lib/types/consult';

export const consult = {
  list: (q: { status?: ChargeStatus; patient_id?: string; encounter_id?: string; appointment_id?: string } = {}) => {
    const p = new URLSearchParams();
    for (const [k, v] of Object.entries(q)) if (v) p.set(k, v);
    const qs = p.toString();
    return request<{ charges: Charge[] }>('GET', `/consult-charges${qs ? `?${qs}` : ''}`).then((r) => r.charges);
  },
  get: (id: string) => request<{ charge: Charge }>('GET', `/consult-charges/${seg(id)}`).then((r) => r.charge),
  catalog: (q = '', kind = '') =>
    request<{ items: ChargeCatalogItem[] }>('GET', `/consult-charges/catalog?q=${seg(q)}&kind=${seg(kind)}`).then((r) => r.items),
  create: (body: ChargeInput) => request<{ charge: Charge }>('POST', '/consult-charges', body).then((r) => r.charge),
  update: (id: string, body: ChargeInput) => request<{ charge: Charge }>('PUT', `/consult-charges/${seg(id)}`, body).then((r) => r.charge),
  send: (id: string) => request<{ charge: Charge }>('POST', `/consult-charges/${seg(id)}/send`).then((r) => r.charge),
  cancel: (id: string, reason = '') => request<{ charge: Charge; consumed_kept: number }>('POST', `/consult-charges/${seg(id)}/cancel`, { reason }),

  complements: (invoiceId: string) => request<PaymentComplements>('GET', `/pos/invoices/${seg(invoiceId)}/payment-complements`),
  issueComplement: (invoiceId: string, paymentId: string) =>
    request<ComplementResult>('POST', `/pos/invoices/${seg(invoiceId)}/payment-complement`, { payment_id: paymentId }),
  complementFileUrl: (invoiceId: string, complementId: string, ext: 'xml' | 'pdf') =>
    `/api/pos/invoices/${seg(invoiceId)}/payment-complements/${seg(complementId)}/${ext}`
};
