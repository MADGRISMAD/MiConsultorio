import { request, seg } from '$lib/api';
import type { InvoiceRequest, Sale, SalePaymentInput } from '$lib/types';
import type {
  ApptPrefill,
  CommissionReport,
  CommissionRule,
  CommissionRuleInput,
  Consumable,
  PlanPrefill,
  PosAlerts,
  Professional,
  Receivables,
  StampResult,
  StockLot,
  VoidResult
} from '$lib/types/pos2';

export const pos2 = {
  lots: (itemId: string) => request<{ lots: StockLot[] }>('GET', `/pos/items/${seg(itemId)}/lots`).then((r) => r.lots),
  alerts: (days = 60) => request<PosAlerts>('GET', `/pos/alerts?days=${days}`),

  consumables: (serviceId: string) => request<{ consumables: Consumable[] }>('GET', `/pos/items/${seg(serviceId)}/consumables`).then((r) => r.consumables),
  saveConsumables: (serviceId: string, items: { product_id: string; qty: number }[]) =>
    request<{ consumables: Consumable[] }>('PUT', `/pos/items/${seg(serviceId)}/consumables`, { items }).then((r) => r.consumables),
  encounterConsumables: (encounterId: string, items: { item_id: string; qty: number }[]) =>
    request<unknown>('POST', `/encounters/${seg(encounterId)}/consumables`, { items }),

  professionals: () => request<{ professionals: Professional[] }>('GET', '/pos/professionals').then((r) => r.professionals),
  rules: () => request<{ rules: CommissionRule[] }>('GET', '/pos/commission-rules').then((r) => r.rules),
  createRule: (r: CommissionRuleInput) => request<{ rule: CommissionRule }>('POST', '/pos/commission-rules', r).then((x) => x.rule),
  updateRule: (id: string, r: CommissionRuleInput) => request<{ rule: CommissionRule }>('PUT', `/pos/commission-rules/${seg(id)}`, r).then((x) => x.rule),
  deleteRule: (id: string) => request<unknown>('DELETE', `/pos/commission-rules/${seg(id)}`),
  commissions: (from = '', to = '', professional = '') =>
    request<CommissionReport>('GET', `/pos/reports/commissions?from=${seg(from)}&to=${seg(to)}&professional=${seg(professional)}`),
  commissionsCsvUrl: (from = '', to = '', professional = '') => `/api/pos/reports/commissions.csv?from=${seg(from)}&to=${seg(to)}&professional=${seg(professional)}`,

  receivables: () => request<Receivables>('GET', '/pos/receivables'),
  addPayments: (saleId: string, payments: SalePaymentInput[]) => request<{ sale: Sale }>('POST', `/pos/sales/${seg(saleId)}/payments`, { payments }).then((r) => r.sale),
  voidSale: (id: string, reason: string) => request<VoidResult>('POST', `/pos/sales/${seg(id)}/void`, { reason }),

  /** invoice requests plus whether the server can stamp CFDI (credentials configured) */
  invoices: (status = '') =>
    request<{ invoices: InvoiceRequest[]; stamping?: { enabled: boolean } }>('GET', `/pos/invoices?status=${seg(status)}`).then((r) => ({
      invoices: r.invoices,
      stamping: !!r.stamping?.enabled
    })),
  stamp: (invoiceId: string) => request<StampResult>('POST', `/pos/invoices/${seg(invoiceId)}/stamp`),
  cancelCfdi: (invoiceId: string, motive: string, replacement_uuid?: string) =>
    request<unknown>('POST', `/pos/invoices/${seg(invoiceId)}/cancel-cfdi`, { motive, replacement_uuid }),
  cfdiFileUrl: (invoiceId: string, ext: 'xml' | 'pdf') => `/api/pos/invoices/${seg(invoiceId)}/${ext}`,

  // data owned by other modules, read defensively
  appointment: (id: string) =>
    request<{ appointment: ApptPrefill }>('GET', `/appointments/${seg(id)}`).then((r) => r.appointment),
  plan: (id: string) => request<{ plan: PlanPrefill }>('GET', `/plans/${seg(id)}`).then((r) => r.plan)
};
