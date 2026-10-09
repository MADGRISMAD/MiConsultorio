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
export const paidCash = (sale: Sale) => (sale.payments ?? []).some((p) => p.method === 'cash');

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
  p.align('center').feed(0).line('Gracias por confiar en nosotros').line('Que te mejores pronto');
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

/** Black-ink styles for the 58 / 80 mm tickets: ticket paper, boxes and bands instead of colour, so any thermal printer renders them. */
function ticketCss(width: number): string {
  const w = width === 58 ? 58 : 80;
  return `
@page { size: ${w}mm auto; margin: 0; }
* { box-sizing: border-box; }
html, body { margin: 0; background: #fff; }
.tk { --pad: ${w === 58 ? '3.5mm' : '4mm'}; width: ${w}mm; padding: 3mm var(--pad) 8mm; color: #000; background: #fff;
  font: ${w === 58 ? '10.5px' : '12px'}/1.35 "Figtree", system-ui, -apple-system, "Segoe UI", Arial, sans-serif; font-variant-numeric: tabular-nums;
  -webkit-print-color-adjust: exact; print-color-adjust: exact; }
.tk p { margin: 0; }
.tk-head { display: grid; justify-items: center; gap: 0.6mm; text-align: center; }
.tk-pulse { display: block; width: 100%; height: auto; margin-bottom: 2mm; }
.tk-name { font-size: 1.7em; font-weight: 900; line-height: 1.05; letter-spacing: -0.01em; text-transform: uppercase; overflow-wrap: anywhere; }
.tk-kind { display: flex; align-items: center; gap: 2mm; width: 100%; margin: 0.6mm 0; font-size: 0.76em; font-weight: 800; letter-spacing: 0.22em; text-transform: uppercase; }
.tk-kind::before, .tk-kind::after { content: ""; flex: 1; border-top: 1px solid #000; }
.tk-kind span { white-space: nowrap; }
.tk-addr { font-size: 0.86em; line-height: 1.3; }
.tk-stub { display: grid; grid-template-columns: minmax(0, 1fr) auto; margin-top: 3mm; border: 1.5px solid #000; border-radius: 1.6mm; }
.tk-stub > div { padding: 1.4mm 2.2mm; min-width: 0; }
.tk-stub > div + div { border-left: 1.5px dashed #000; text-align: right; }
.tk-stub span { display: block; font-size: 0.7em; font-weight: 800; letter-spacing: 0.18em; text-transform: uppercase; }
.tk-stub strong { display: block; font-size: 1.5em; font-weight: 900; line-height: 1.1; white-space: nowrap; }
.tk-stub .d { font-size: 1em; font-weight: 800; line-height: 1.25; }
.tk-meta { margin-top: 1.8mm; font-size: 0.92em; display: grid; gap: 0.4mm; }
.tk-meta b { font-weight: 800; text-transform: uppercase; font-size: 0.82em; letter-spacing: 0.08em; }
.tk-sec { display: flex; align-items: center; gap: 2mm; margin: 3.5mm 0 2mm; font-size: 0.76em; font-weight: 900; letter-spacing: 0.2em; text-transform: uppercase; white-space: nowrap; }
.tk-sec::before, .tk-sec::after { content: ""; flex: 1; border-top: 1.5px solid #000; }
.tk-sec::before { flex: 0 0 3mm; }
.tk-items { display: grid; gap: 2mm; }
.tk-item { display: grid; grid-template-columns: 5.4mm minmax(0, 1fr); column-gap: 1.2mm; align-items: start; }
.tk-n { display: grid; place-items: center; height: 4.2mm; margin-top: 0.2mm; border: 1.2px solid #000; border-radius: 1mm; font-size: 0.72em; font-weight: 800; }
.tk-iname { font-weight: 800; line-height: 1.2; overflow-wrap: anywhere; }
.tk-line { display: flex; align-items: baseline; gap: 1mm; min-width: 0; }
.tk-line > .dots { flex: 1 1 4mm; min-width: 4mm; border-bottom: 1.5px dotted #000; transform: translateY(-0.6mm); }
.tk-line > :first-child { min-width: 0; overflow-wrap: anywhere; }
.tk-line > :last-child { flex-shrink: 0; text-align: right; white-space: nowrap; }
.tk-line + .tk-line { margin-top: 0.8mm; }
.tk-item .tk-line { margin-top: 0.4mm; font-size: 0.95em; }
.tk-line.strong { font-weight: 800; }
.tk-sum { margin-top: 2.5mm; display: grid; gap: 0.8mm; }
.tk-total { display: flex; align-items: center; justify-content: space-between; gap: 2mm; margin: 3mm calc(-1 * var(--pad) + 1mm) 0; padding: 2mm 2.5mm; background: #000; color: #fff; border-radius: 1.2mm; }
.tk-total span { font-size: 0.95em; font-weight: 900; letter-spacing: 0.22em; text-transform: uppercase; }
.tk-total strong { font-size: 2em; font-weight: 900; line-height: 1; letter-spacing: -0.02em; white-space: nowrap; }
.tk-pay { display: grid; grid-template-columns: minmax(0, 1.15fr) minmax(0, 1fr); margin-top: 1.5mm; border: 1.5px solid #000; border-radius: 1.2mm; }
.tk-pay > div { padding: 1.4mm 2mm; min-width: 0; }
.tk-pay > div + div { border-left: 1.5px solid #000; text-align: right; }
.tk-pay.single { grid-template-columns: minmax(0, 1fr); }
.tk-pay span { display: block; font-size: 0.7em; font-weight: 800; letter-spacing: 0.16em; text-transform: uppercase; }
.tk-pay strong { display: block; font-size: 1.05em; font-weight: 800; overflow-wrap: anywhere; }
.tk-pay .ch strong { font-size: 1.4em; font-weight: 900; line-height: 1.1; }
.tk-flag { margin-top: 2mm; padding: 1.2mm 2mm; border: 1.5px dashed #000; border-radius: 1.2mm; text-align: center; font-size: 0.92em; font-weight: 800; }
.tk-stamp-row { display: flex; justify-content: center; margin: 5mm 0 2mm; }
.tk-stamp { display: grid; justify-items: center; gap: 0.4mm; padding: 1.2mm 4mm 1.4mm; border: 4px double #000; border-radius: 2.2mm; transform: rotate(-5deg); text-align: center; }
.tk-stamp b { font-size: 1.5em; font-weight: 900; line-height: 1; letter-spacing: 0.2em; margin-right: -0.2em; text-transform: uppercase; }
.tk-stamp small { font-size: 0.72em; font-weight: 800; letter-spacing: 0.2em; text-transform: uppercase; }
.tk-foot { margin-top: 6mm; text-align: center; font-size: 0.9em; }
.tk-thanks { margin-top: 2mm; text-align: center; font-weight: 900; font-size: 1.05em; }
.tk-care { display: block; margin: 0 auto 1.5mm; }
.tk-note { margin-top: 1mm; text-align: center; font-size: 0.82em; }
.tk-sign { margin-top: 9mm; text-align: center; }
.tk-sign i { display: block; border-top: 1.5px solid #000; margin: 0 6mm 1mm; }
`;
}

/** An ECG line with a medical cross: the header's health motif, drawn in solid black. */
const PULSE_SVG = `<svg class="tk-pulse" viewBox="0 0 160 22" aria-hidden="true"><path d="M0 12h52l5-9 7 17 6-13 4 5h86" fill="none" stroke="#000" stroke-width="2" stroke-linejoin="round" stroke-linecap="round"/></svg>`;
const CROSS_SVG = `<svg class="tk-care" width="20" height="20" viewBox="0 0 20 20" aria-hidden="true"><path d="M7 1h6v6h6v6h-6v6H7v-6H1V7h6z" fill="#000"/></svg>`;

const ticketHeader = (s: PosSettings) =>
  `<header class="tk-head">${PULSE_SVG}
    <p class="tk-name">${esc(s.business_name || 'Mi consultorio')}</p>
    <p class="tk-kind"><span>Consultorio</span></p>
    ${s.legal_name && s.legal_name !== s.business_name ? `<p class="tk-addr">${esc(s.legal_name)}</p>` : ''}
    ${s.rfc ? `<p class="tk-addr">RFC: ${esc(s.rfc)}</p>` : ''}
    ${s.tax_address ? `<p class="tk-addr">${esc(s.tax_address)}${s.zip_code ? ', C.P. ' + esc(s.zip_code) : ''}</p>` : ''}
    ${s.phone ? `<p class="tk-addr">Tel. ${esc(s.phone)}</p>` : ''}
    ${s.ticket_header ? `<p class="tk-addr">${esc(s.ticket_header).replace(/\n/g, '<br>')}</p>` : ''}
  </header>`;

const dateParts = (iso: string) => {
  const d = new Date(iso);
  return {
    day: d.toLocaleDateString('es-MX', { day: '2-digit', month: 'short', year: 'numeric' }),
    time: d.toLocaleTimeString('es-MX', { hour: '2-digit', minute: '2-digit', hour12: false })
  };
};

const dotted = (l: string, r: string, cls = '') => `<div class="tk-line ${cls}"><span>${esc(l)}</span><i class="dots"></i><span>${esc(r)}</span></div>`;
const wrapTicket = (title: string, width: number, inner: string) =>
  `<!doctype html><html lang="es"><head><meta charset="utf-8"><title>${esc(title)}</title><style>${ticketCss(width)}</style></head><body><main class="tk">${inner}</main></body></html>`;

export function ticketHtml(sale: Sale, s: PosSettings, opts: { test?: boolean; reprint?: boolean } = {}): string {
  if (opts.test) {
    return wrapTicket('Prueba', s.printer.width, `${ticketHeader(s)}<div class="tk-flag">PRUEBA DE IMPRESIÓN</div><p class="tk-foot">Si lees esto con acentos (áéíóú ñ ¿?)<br>tu impresora está lista.</p>`);
  }
  const itemDisc = (sale.lines ?? []).reduce((a, l) => a + l.discount_cents, 0);
  const extraDisc = sale.discount_cents - itemDisc;
  const dt = dateParts(sale.created_at);
  const items = (sale.lines ?? [])
    .map(
      (l, i) => `<div class="tk-item"><span class="tk-n">${i + 1}</span><div>
        <div class="tk-iname">${esc(l.name)}</div>
        ${dotted(`${qty(l.qty)} × ${peso(l.unit_price_cents)}`, peso(l.total_cents + l.discount_cents))}
        ${l.discount_cents ? dotted('Descuento', '-' + peso(l.discount_cents)) : ''}</div></div>`
    )
    .join('');
  const pays = (sale.payments ?? [])
    .map((p) => {
      const method = PAY_METHODS[p.method]?.label.replace(/ \(.*\)/, '') ?? p.method;
      const change = p.received_cents && p.change_cents > 0;
      return `<div class="tk-pay ${change ? '' : 'single'}"><div><span>Pago · ${esc(method)}</span><strong>${peso(change ? (p.received_cents ?? 0) : p.amount_cents)}</strong>${p.reference ? `<em>Ref: ${esc(p.reference)}</em>` : ''}</div>${change ? `<div class="ch"><span>Cambio</span><strong>${peso(p.change_cents)}</strong></div>` : ''}</div>`;
    })
    .join('');
  const void_ = sale.status === 'void';
  const owes = (sale.balance_cents ?? 0) > 0;
  const stamp = void_ ? ['Cancelado', dt.day] : owes ? ['Por cobrar', 'Saldo pendiente'] : ['Pagado', `${dt.day} · ${dt.time}`];
  const inner = `${ticketHeader(s)}
    <div class="tk-stub"><div><span>Folio</span><strong>#${sale.folio}</strong></div><div><span>Fecha</span><strong class="d">${esc(dt.day)}<br>${esc(dt.time)}</strong></div></div>
    ${opts.reprint ? '<div class="tk-flag">*** REIMPRESIÓN ***</div>' : ''}
    ${sale.customer_name || sale.created_by ? `<div class="tk-meta">${sale.customer_name ? `<div><b>Paciente</b> ${esc(sale.customer_name)}</div>` : ''}${sale.created_by ? `<div><b>Atendió</b> ${esc(sale.created_by)}</div>` : ''}</div>` : ''}
    <div class="tk-sec">Detalle</div>
    <div class="tk-items">${items}</div>
    <div class="tk-sum">${extraDisc > 0 ? dotted('Descuento', '-' + peso(extraDisc)) : ''}${s.show_tax_line && sale.tax_cents > 0 ? dotted('IVA incluido', peso(sale.tax_cents)) : ''}</div>
    <div class="tk-total"><span>Total</span><strong>${peso(sale.total_cents)}</strong></div>
    ${pays}
    ${owes ? `<div class="tk-sum">${dotted('Abonado', peso(sale.paid_cents ?? 0))}${dotted('Saldo pendiente', peso(sale.balance_cents ?? 0), 'strong')}</div>` : ''}
    <div class="tk-stamp-row"><div class="tk-stamp"><b>${esc(stamp[0])}</b><small>${esc(stamp[1])}</small></div></div>
    ${s.ticket_footer ? `<p class="tk-foot">${esc(s.ticket_footer).replace(/\n/g, '<br>')}</p>` : ''}
    <p class="tk-thanks">${CROSS_SVG}¡Gracias por confiar en nosotros!<br>Que te mejores pronto.</p>
    <p class="tk-note">Comprobante de pago. No sustituye una factura (CFDI).</p>`;
  return wrapTicket('Ticket #' + sale.folio, s.printer.width, inner);
}

/** The receipt on a letter-size sheet (comprobante de pago "carta"). It is not a tax invoice. */
export function cartaHtml(sale: Sale, s: PosSettings): string {
  const itemDisc = (sale.lines ?? []).reduce((a, l) => a + l.discount_cents, 0);
  const extraDisc = sale.discount_cents - itemDisc;
  const lines = (sale.lines ?? [])
    .map((l) => `<tr><td class="r">${esc(qty(l.qty))}</td><td>${esc(l.name)}</td><td class="r">${peso(l.unit_price_cents)}</td><td class="r">${l.discount_cents ? '-' + peso(l.discount_cents) : ''}</td><td class="r">${peso(l.total_cents)}</td></tr>`)
    .join('');
  const pays = (sale.payments ?? [])
    .map((p) => `<tr><td>${esc(PAY_METHODS[p.method]?.label.replace(/ \(.*\)/, '') ?? p.method)}${p.reference ? ` <span class="s">Ref: ${esc(p.reference)}</span>` : ''}</td><td class="r">${peso(p.amount_cents)}</td></tr>`)
    .join('');
  const total = (l: string, v: string, b = false) => `<tr class="${b ? 'b' : ''}"><td>${l}</td><td class="r">${v}</td></tr>`;
  return `<!doctype html><html lang="es"><head><meta charset="utf-8"><title>Comprobante #${sale.folio}</title>
<style>
@page { size: letter; margin: 18mm; }
* { box-sizing: border-box; }
body { font: 13px/1.45 Arial, Helvetica, sans-serif; color: #000; margin: 0; }
h1 { font-size: 22px; margin: 0 0 2px; } .s { font-size: 11px; color: #444; }
.head { display: flex; justify-content: space-between; gap: 20px; border-bottom: 2px solid #000; padding-bottom: 10px; margin-bottom: 14px; }
.doc { text-align: right; }
table { width: 100%; border-collapse: collapse; margin-top: 8px; }
th { text-align: left; background: #eee; padding: 5px 8px; border: 1px solid #000; font-size: 12px; }
td { padding: 5px 8px; border: 1px solid #bbb; vertical-align: top; }
.r { text-align: right; white-space: nowrap; } th.r { text-align: right; }
.tot { width: 55%; margin-left: auto; } .tot td { border: 0; padding: 3px 8px; } .b td { font-weight: 700; font-size: 15px; border-top: 2px solid #000; }
.foot { margin-top: 26px; font-size: 11px; color: #333; text-align: center; border-top: 1px solid #999; padding-top: 8px; }
</style></head><body>
<div class="head">
  <div><h1>${esc(s.business_name || 'Mi consultorio')}</h1>
    ${s.legal_name && s.legal_name !== s.business_name ? `<div>${esc(s.legal_name)}</div>` : ''}
    ${s.rfc ? `<div>RFC: ${esc(s.rfc)}</div>` : ''}
    ${s.tax_address ? `<div class="s">${esc(s.tax_address)}${s.zip_code ? ', C.P. ' + esc(s.zip_code) : ''}</div>` : ''}
    ${s.phone ? `<div class="s">Tel. ${esc(s.phone)}</div>` : ''}</div>
  <div class="doc"><div><strong>Comprobante de pago</strong></div><div>Folio #${sale.folio}</div><div class="s">${esc(when(sale.created_at))}</div>
    ${sale.status === 'void' ? '<div><strong>CANCELADO</strong></div>' : ''}</div>
</div>
${sale.customer_name ? `<div><strong>Paciente:</strong> ${esc(sale.customer_name)}</div>` : ''}
${sale.created_by ? `<div><strong>Atendió:</strong> ${esc(sale.created_by)}</div>` : ''}
<table><thead><tr><th class="r" style="width:60px">Cant.</th><th>Concepto</th><th class="r">Precio</th><th class="r">Desc.</th><th class="r">Importe</th></tr></thead><tbody>${lines}</tbody></table>
<table class="tot"><tbody>
${extraDisc > 0 ? total('Descuento', '-' + peso(extraDisc)) : ''}
${s.show_tax_line && sale.tax_cents > 0 ? total('IVA incluido', peso(sale.tax_cents)) : ''}
${total('Total', peso(sale.total_cents), true)}
</tbody></table>
${pays ? `<table class="tot"><tbody><tr><td colspan="2"><strong>Forma de pago</strong></td></tr>${pays}${(sale.balance_cents ?? 0) > 0 ? total('Saldo pendiente', peso(sale.balance_cents ?? 0), true) : ''}</tbody></table>` : ''}
<div class="foot">${s.ticket_footer ? esc(s.ticket_footer).replace(/\n/g, '<br>') + '<br>' : ''}Este documento es un comprobante de pago y no sustituye a una factura (CFDI).</div>
</body></html>`;
}

/** The return note (nota de devolución) for the browser's print dialog, in the same ticket style. */
export function returnTicketHtml(ret: ReturnResult, s: PosSettings): string {
  const dt = dateParts(ret.created_at);
  const inner = `${ticketHeader(s)}
    <div class="tk-stub"><div><span>Devolución</span><strong>#${ret.folio}</strong></div><div><span>Fecha</span><strong class="d">${esc(dt.day)}<br>${esc(dt.time)}</strong></div></div>
    <div class="tk-meta"><div><b>Venta</b> #${ret.sale_folio}</div>${ret.created_by_name ? `<div><b>Atendió</b> ${esc(ret.created_by_name)}</div>` : ''}</div>
    <div class="tk-sec">Devuelto</div>
    <div class="tk-items">${ret.lines
      .map((l, i) => `<div class="tk-item"><span class="tk-n">${i + 1}</span><div><div class="tk-iname">${esc(l.name)}</div>${dotted(`${qty(l.qty)} ×`, peso(l.amount_cents))}</div></div>`)
      .join('')}</div>
    <div class="tk-total"><span>Devuelto</span><strong>${peso(ret.total_cents)}</strong></div>
    ${ret.refunds
      .map((r) => `<div class="tk-pay single"><div><span>Devuelto en · ${esc(PAY_METHODS[r.method as keyof typeof PAY_METHODS]?.label.replace(/ \(.*\)/, '') ?? r.method)}</span><strong>${peso(r.amount_cents)}</strong></div></div>`)
      .join('')}
    <p class="tk-foot"><b>Motivo:</b> ${esc(ret.reason)}</p>
    <div class="tk-sign"><i></i>Firma de quien recibe</div>`;
  return wrapTicket('Devolución #' + ret.folio, s.printer.width, inner);
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
