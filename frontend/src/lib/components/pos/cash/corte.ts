import { PAY_METHODS, type CashSession } from '$lib/types';
import { esc } from '../sale/money';

const peso = (cents: number) => (cents / 100).toLocaleString('es-MX', { style: 'currency', currency: 'MXN' });
const when = (iso: string | null) =>
  iso ? new Date(iso).toLocaleString('es-MX', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false }) : '—';

/** A small printable "corte de caja" (works on a thermal roll or a normal sheet). */
export function corteHtml(s: CashSession, businessName: string, widthMm: 58 | 80 = 80): string {
  const row = (l: string, r: string, cls = '') => `<div class="row ${cls}"><span>${esc(l)}</span><span>${esc(r)}</span></div>`;
  const diff = s.diff_cents ?? 0;
  const moves = (s.movements ?? [])
    .map((m) => row(`${m.kind === 'in' ? '+' : '−'} ${m.concept}`, peso(m.amount_cents), 'sub'))
    .join('');
  const body = `<hr><div class="c b">CORTE DE CAJA</div><hr>
    ${row('Apertura', when(s.opened_at))}${s.opened_by ? `<div class="sub">Abrió: ${esc(s.opened_by)}</div>` : ''}
    ${row('Cierre', when(s.closed_at))}${s.closed_by ? `<div class="sub">Cerró: ${esc(s.closed_by)}</div>` : ''}<hr>
    ${row('Fondo inicial', peso(s.opening_cents))}
    ${row(`Ventas (${s.sales})`, peso(s.sales_cents), 'b')}
    ${s.by_method.map((m) => row(`  ${(PAY_METHODS[m.method]?.label ?? m.method).replace(/ \(.*\)/, '')} (${m.count})`, peso(m.amount_cents), 'sub')).join('')}<hr>
    ${row('Efectivo de ventas', peso(s.cash_in_cents))}
    ${row('Entradas de efectivo', peso(s.moves_in_cents))}
    ${row('Salidas de efectivo', '-' + peso(s.moves_out_cents))}
    ${moves}<hr>
    ${row('Efectivo esperado', peso(s.expected_cents), 'b')}
    ${s.counted_cents != null ? row('Efectivo contado', peso(s.counted_cents), 'b') : ''}
    ${s.diff_cents != null ? row(diff === 0 ? 'Diferencia' : diff > 0 ? 'Sobrante' : 'Faltante', peso(Math.abs(diff)), 'b big') : ''}
    ${s.note ? `<hr><div>Nota: ${esc(s.note)}</div>` : ''}<hr>
    <div class="c">Firma: ____________________</div>`;
  const w = widthMm === 58 ? '54mm' : '76mm';
  return `<!doctype html><html lang="es"><head><meta charset="utf-8"><title>Corte de caja</title>
<style>
@page { size: ${widthMm}mm auto; margin: 2mm; }
* { box-sizing: border-box; }
body { width: ${w}; margin: 0 auto; font: 12px/1.35 'Courier New', ui-monospace, monospace; color: #000; }
.c { text-align: center; } .b { font-weight: 700; } .big { font-size: 15px; }
.row { display: flex; justify-content: space-between; gap: 8px; } .row span:first-child { overflow-wrap: anywhere; }
.sub { padding-left: 4px; font-size: 11px; }
hr { border: 0; border-top: 1px dashed #000; margin: 5px 0; }
</style></head><body><div class="c b big">${esc(businessName || 'Mi consultorio')}</div>${body}</body></html>`;
}
