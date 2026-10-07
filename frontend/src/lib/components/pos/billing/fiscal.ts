import type { InvoiceRequest } from '$lib/types';
import { moneyCents } from '$lib/format';

/** Common SAT tax regimes (clave and name). */
export const TAX_REGIMES: { id: string; name: string }[] = [
  { id: '601', name: 'General de Ley Personas Morales' },
  { id: '603', name: 'Personas Morales con Fines no Lucrativos' },
  { id: '605', name: 'Sueldos y Salarios e Ingresos Asimilados a Salarios' },
  { id: '606', name: 'Arrendamiento' },
  { id: '612', name: 'Personas Físicas con Actividades Empresariales y Profesionales' },
  { id: '616', name: 'Sin obligaciones fiscales' },
  { id: '621', name: 'Incorporación Fiscal' },
  { id: '626', name: 'Régimen Simplificado de Confianza (RESICO)' }
];

/** Common SAT CFDI uses. */
export const CFDI_USES: { id: string; name: string }[] = [
  { id: 'G01', name: 'Adquisición de mercancías' },
  { id: 'G03', name: 'Gastos en general' },
  { id: 'D01', name: 'Honorarios médicos, dentales y gastos hospitalarios' },
  { id: 'D02', name: 'Gastos médicos por incapacidad o discapacidad' },
  { id: 'D07', name: 'Primas por seguros de gastos médicos' },
  { id: 'S01', name: 'Sin efectos fiscales' }
];

export const regimeLabel = (id: string) => {
  const r = TAX_REGIMES.find((x) => x.id === id);
  return r ? `${r.id} · ${r.name}` : id || '—';
};
export const cfdiUseLabel = (id: string) => {
  const r = CFDI_USES.find((x) => x.id === id);
  return r ? `${r.id} · ${r.name}` : id || '—';
};

export const RFC_RE = /^[A-ZÑ&]{3,4}\d{6}[A-Z0-9]{3}$/i;
export const GENERIC_RFC = 'XAXX010101000';
export const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
export const ZIP_RE = /^\d{5}$/;

export const INVOICE_STATUS: Record<InvoiceRequest['status'], { label: string; tone: 'warn' | 'ok' | 'muted' }> = {
  pending: { label: 'Pendiente', tone: 'warn' },
  issued: { label: 'Emitida', tone: 'ok' },
  cancelled: { label: 'Cancelada', tone: 'muted' }
};

/** A tidy text block the accountant can paste into the PAC. */
export function invoiceText(i: InvoiceRequest): string {
  return [
    `Solicitud de factura · Venta #${i.folio}`,
    `RFC: ${i.rfc}`,
    `Razón social: ${i.legal_name}`,
    `Régimen fiscal: ${regimeLabel(i.tax_regime)}`,
    `Código postal fiscal: ${i.zip_code}`,
    `Uso de CFDI: ${cfdiUseLabel(i.cfdi_use)}`,
    `Correo: ${i.email || '—'}`,
    `Total: ${moneyCents(i.total_cents)} MXN (IVA incluido)`,
    i.fiscal_uuid ? `Folio fiscal (UUID): ${i.fiscal_uuid}` : ''
  ]
    .filter(Boolean)
    .join('\n');
}
