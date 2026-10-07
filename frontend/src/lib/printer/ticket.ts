import type { PosSettings, Sale } from '$lib/types';
import { PAY_METHODS } from '$lib/types';
import type { ReturnResult } from '$lib/types/pos2';
import { EscPos, wrap } from './escpos';

const peso = (cents: number) =>
  (cents / 100).toLocaleString('es-MX', { style: 'currency', currency: 'MXN' });

const qty = (n: number) => (Number.isInteger(n) ? String(n) : n.toLocaleString('es-MX', { maximumFractionDigits: 3 }));

const when = (iso: string) =>
  new Date(iso).toLocaleString('es-MX', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false });

/** Cash drawer pulses only when something was paid in cash. */
const paidCash = (sale: Sale) => (sale.payments ?? []).some((p) => p.method === 'cash');

/** The ticket as ESC/POS bytes for a thermal printer. */
export function ticketBytes(sale: Sale, s: PosSettings, opts: { test?: boolean; reprint?: boolean } = {}): Uint8Array {
  const p = new EscPos(s.printer.width);
  p.align('center').bold(true).size(2).line(s.business_name || 'Mi consultorio').size(1).bold(false);
  if (s.legal_name && s.legal_name !== s.business_name) p.line(s.legal_name);
  if (s.rfc) p.line('RFC: ' + s.rfc);
  for (const l of wrap([s.tax_address, s.zip_code && 'C.P. ' + s.zip_code].filter(Boolean).join(', '), p.cols)) if (l) p.line(l);
  if (s.phone) p.line('Tel. ' + s.phone);
  if (s.ticket_header) for (const l of wrap(s.ticket_header, p.cols)) p.line(l);
  p.align('left').rule();
  if (opts.test) {
    p.align('center').bold(true).line('PRUEBA DE IMPRESIÓN').bold(false).line('Si lees esto con acentos (áéíóú ñ ¿?)').line('tu impresora está lista.').rule();
    p.feed(1).cut();
    return p.build();
  }
  p.pair('Folio: #' + sale.folio, when(sale.created_at));
  if (opts.reprint) p.align('center').bold(true).line('*** REIMPRESIÓN ***').bold(false).align('left');
  if (sale.status === 'void') p.align('center').bold(true).line('*** VENTA CANCELADA ***').bold(false).align('left');
  if (sale.customer_name) for (const l of wrap('Paciente: ' + sale.customer_name, p.cols)) p.line(l);
  if (sale.created_by) p.line('Atendió: ' + sale.created_by);
  p.rule();
  for (const l of sale.lines ?? []) {
    p.pair(`${qty(l.qty)} x ${l.name}`, peso(l.total_cents + l.discount_cents));
    if (l.qty !== 1) p.line('   @ ' + peso(l.unit_price_cents));
    if (l.discount_cents) p.pair('   Descuento', '-' + peso(l.discount_cents));
  }
  p.rule();
  const itemDisc = (sale.lines ?? []).reduce((a, l) => a + l.discount_cents, 0);
  const extraDisc = sale.discount_cents - itemDisc;
  if (extraDisc > 0) p.pair('Descuento', '-' + peso(extraDisc));
  p.bold(true).size(2);
  // double size halves the usable columns
  p.line(totalLine('TOTAL', peso(sale.total_cents), Math.floor(p.cols / 2)));
  p.size(1).bold(false);
  if (s.show_tax_line && sale.tax_cents > 0) p.pair('IVA incluido', peso(sale.tax_cents));
  p.rule();
  for (const pay of sale.payments ?? []) {
    p.pair(PAY_METHODS[pay.method]?.label.replace(/ \(.*\)/, '') ?? pay.method, peso(pay.amount_cents));
    if (pay.received_cents && pay.change_cents > 0) {
      p.pair('  Recibido', peso(pay.received_cents));
      p.pair('  Cambio', peso(pay.change_cents));
    }
    if (pay.reference) p.line('  Ref: ' + pay.reference);
  }
  if ((sale.balance_cents ?? 0) > 0) {
    p.rule();
    p.pair('Abonado', peso(sale.paid_cents ?? 0));
    p.bold(true).pair('SALDO PENDIENTE', peso(sale.balance_cents ?? 0)).bold(false);
  }
  if (s.ticket_footer) {
    p.rule().align('center');
    for (const l of wrap(s.ticket_footer, p.cols)) p.line(l);
  }
  p.align('center').feed(0).line('Gracias por su preferencia');
  if (s.printer.open_drawer && paidCash(sale) && !opts.reprint) p.drawer();
  if (s.printer.cut) p.cut();
  else p.feed(4);
  return p.build();
}

function totalLine(label: string, amount: string, width: number): string {
  const space = Math.max(1, width - label.length - amount.length);
  return label + ' '.repeat(space) + amount;
}

// ---------------------------------------------------------------------------
// HTML version (any printer through the browser's print dialog)
// ---------------------------------------------------------------------------

const esc = (s: string) => s.replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]!);

export function ticketHtml(sale: Sale, s: PosSettings, opts: { test?: boolean; reprint?: boolean } = {}): string {
  const w = s.printer.width === 58 ? '54mm' : '76mm';
  const row = (l: string, r: string, cls = '') => `<div class="row ${cls}"><span>${esc(l)}</span><span>${esc(r)}</span></div>`;
  const head = [
    `<div class="c b big">${esc(s.business_name || 'Mi consultorio')}</div>`,
    s.legal_name && s.legal_name !== s.business_name ? `<div class="c">${esc(s.legal_name)}</div>` : '',
    s.rfc ? `<div class="c">RFC: ${esc(s.rfc)}</div>` : '',
    s.tax_address ? `<div class="c">${esc(s.tax_address)}${s.zip_code ? ', C.P. ' + esc(s.zip_code) : ''}</div>` : '',
    s.phone ? `<div class="c">Tel. ${esc(s.phone)}</div>` : '',
    s.ticket_header ? `<div class="c">${esc(s.ticket_header).replace(/\n/g, '<br>')}</div>` : ''
  ].join('');
  let body: string;
  if (opts.test) {
    body = `<hr><div class="c b">PRUEBA DE IMPRESIÓN</div><div class="c">Si lees esto con acentos (áéíóú ñ ¿?)<br>tu impresora está lista.</div><hr>`;
  } else {
    const itemDisc = (sale.lines ?? []).reduce((a, l) => a + l.discount_cents, 0);
    const extraDisc = sale.discount_cents - itemDisc;
    body = `<hr>${row('Folio: #' + sale.folio, when(sale.created_at))}
      ${opts.reprint ? '<div class="c b">*** REIMPRESIÓN ***</div>' : ''}
      ${sale.status === 'void' ? '<div class="c b">*** VENTA CANCELADA ***</div>' : ''}
      ${sale.customer_name ? `<div>Paciente: ${esc(sale.customer_name)}</div>` : ''}
      ${sale.created_by ? `<div>Atendió: ${esc(sale.created_by)}</div>` : ''}<hr>
      ${(sale.lines ?? [])
        .map(
          (l) =>
            row(`${qty(l.qty)} x ${l.name}`, peso(l.total_cents + l.discount_cents)) +
            (l.qty !== 1 ? `<div class="sub">@ ${peso(l.unit_price_cents)}</div>` : '') +
            (l.discount_cents ? row('  Descuento', '-' + peso(l.discount_cents), 'sub') : '')
        )
        .join('')}<hr>
      ${extraDisc > 0 ? row('Descuento', '-' + peso(extraDisc)) : ''}
      ${row('TOTAL', peso(sale.total_cents), 'b big')}
      ${s.show_tax_line && sale.tax_cents > 0 ? row('IVA incluido', peso(sale.tax_cents)) : ''}<hr>
      ${(sale.payments ?? [])
        .map(
          (p) =>
            row(PAY_METHODS[p.method]?.label.replace(/ \(.*\)/, '') ?? p.method, peso(p.amount_cents)) +
            (p.received_cents && p.change_cents > 0 ? row('  Recibido', peso(p.received_cents), 'sub') + row('  Cambio', peso(p.change_cents), 'sub') : '') +
            (p.reference ? `<div class="sub">Ref: ${esc(p.reference)}</div>` : '')
        )
        .join('')}
      ${(sale.balance_cents ?? 0) > 0 ? `<hr>${row('Abonado', peso(sale.paid_cents ?? 0))}${row('SALDO PENDIENTE', peso(sale.balance_cents ?? 0), 'b')}` : ''}
      ${s.ticket_footer ? `<hr><div class="c">${esc(s.ticket_footer).replace(/\n/g, '<br>')}</div>` : ''}
      <div class="c">Gracias por su preferencia</div>`;
  }
  return `<!doctype html><html lang="es"><head><meta charset="utf-8"><title>${opts.test ? 'Prueba' : 'Ticket #' + sale.folio}</title>
<style>
@page { size: ${s.printer.width}mm auto; margin: 2mm; }
* { box-sizing: border-box; }
body { width: ${w}; margin: 0 auto; font: 12px/1.35 'Courier New', ui-monospace, monospace; color: #000; }
.c { text-align: center; } .b { font-weight: 700; } .big { font-size: 15px; }
.row { display: flex; justify-content: space-between; gap: 8px; } .row span:first-child { overflow-wrap: anywhere; }
.sub { padding-left: 4px; font-size: 11px; }
hr { border: 0; border-top: 1px dashed #000; margin: 5px 0; }
</style></head><body>${head}${body}</body></html>`;
}

/** The return note (nota de devolución) for the browser's print dialog. */
export function returnTicketHtml(ret: ReturnResult, s: PosSettings): string {
  const w = s.printer.width === 58 ? '54mm' : '76mm';
  const row = (l: string, r: string, cls = '') => `<div class="row ${cls}"><span>${esc(l)}</span><span>${esc(r)}</span></div>`;
  const head = [
    `<div class="c b big">${esc(s.business_name || 'Mi consultorio')}</div>`,
    s.rfc ? `<div class="c">RFC: ${esc(s.rfc)}</div>` : '',
    s.phone ? `<div class="c">Tel. ${esc(s.phone)}</div>` : ''
  ].join('');
  const body = `<hr><div class="c b">NOTA DE DEVOLUCIÓN #${ret.folio}</div>
    ${row('Venta #' + ret.sale_folio, when(ret.created_at))}
    ${ret.created_by_name ? `<div>Atendió: ${esc(ret.created_by_name)}</div>` : ''}<hr>
    ${ret.lines.map((l) => row(`${qty(l.qty)} x ${l.name}`, peso(l.amount_cents))).join('')}<hr>
    ${row('TOTAL DEVUELTO', peso(ret.total_cents), 'b big')}<hr>
    ${ret.refunds.map((r) => row('Devuelto en ' + (PAY_METHODS[r.method as keyof typeof PAY_METHODS]?.label.replace(/ \(.*\)/, '') ?? r.method), peso(r.amount_cents))).join('')}
    <div class="sub">Motivo: ${esc(ret.reason)}</div>
    <br><br><div class="c">______________________</div><div class="c">Firma de quien recibe</div>`;
  return `<!doctype html><html lang="es"><head><meta charset="utf-8"><title>Devolución #${ret.folio}</title>
<style>
@page { size: ${s.printer.width}mm auto; margin: 2mm; }
* { box-sizing: border-box; }
body { width: ${w}; margin: 0 auto; font: 12px/1.35 'Courier New', ui-monospace, monospace; color: #000; }
.c { text-align: center; } .b { font-weight: 700; } .big { font-size: 15px; }
.row { display: flex; justify-content: space-between; gap: 8px; } .row span:first-child { overflow-wrap: anywhere; }
.sub { padding-left: 4px; font-size: 11px; }
hr { border: 0; border-top: 1px dashed #000; margin: 5px 0; }
</style></head><body>${head}${body}</body></html>`;
}

/** Prints through the browser's own dialog, using a hidden frame. */
export function printHtml(html: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const frame = document.createElement('iframe');
    frame.setAttribute('aria-hidden', 'true');
    Object.assign(frame.style, { position: 'fixed', right: '0', bottom: '0', width: '0', height: '0', border: '0' });
    frame.srcdoc = html;
    frame.onload = () => {
      try {
        frame.contentWindow!.focus();
        frame.contentWindow!.print();
        setTimeout(() => frame.remove(), 1500);
        resolve();
      } catch (e) {
        frame.remove();
        reject(e);
      }
    };
    document.body.appendChild(frame);
  });
}
