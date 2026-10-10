import { FLAG_LABEL, FLAG_MARK, rangeText } from '$lib/components/lab/labUtil';
import type { Issuer, Patient } from '$lib/types';
import type { LabCatalog, LabOrder, LabResult } from '$lib/types/lab';
import { doc, e, fmtDate, fmtDateTime, FOOTER_CONF, issuerBlock, patientBlock } from './base';

const STATUS: Record<string, string> = { solicitado: 'Solicitado', parcial: 'Parcial', completo: 'Completo', cancelado: 'Cancelado' };

function valueText(r: LabResult): string {
  const v = r.value_num != null ? String(r.value_num) : r.value_text;
  return `${v}${r.unit ? ` ${r.unit}` : ''}`;
}

function orderHtml(o: LabOrder): string {
  const current = o.results.filter((r) => !r.superseded_by);
  const panels = new Map<string, LabResult[]>();
  for (const r of current) panels.set(r.panel || 'Otros análisis', [...(panels.get(r.panel || 'Otros análisis') ?? []), r]);
  const table = (rows: LabResult[]) =>
    `<table class="grid-t"><thead><tr><th>Análisis</th><th>Resultado</th><th>Referencia</th><th>Indicador</th></tr></thead><tbody>${rows
      .map((r) => {
        const range = rangeText(r.ref_low, r.ref_high);
        const src = range ? (r.ref_source === 'catalogo' ? ' (general)' : r.ref_source === 'laboratorio' ? ' (del laboratorio)' : '') : '';
        const corr = r.supersedes_id ? '<div class="small">Corrección de un resultado anterior</div>' : '';
        return `<tr class="nobreak"><td>${e(r.analyte)}${corr}</td><td><strong>${e(valueText(r))}</strong></td><td>${e(range)}${e(src)}</td><td>${r.flag === 'na' ? '—' : r.flag === 'normal' ? e(FLAG_LABEL.normal) : `<strong>${e(FLAG_MARK[r.flag])} ${e(FLAG_LABEL[r.flag])}</strong>`}</td></tr>`;
      })
      .join('')}</tbody></table>`;
  const body = [...panels.entries()].map(([name, rows]) => `<h3 style="margin:10px 0 3px">${e(name)}</h3>${table(rows)}`).join('') || '<p class="small">Sin resultados capturados.</p>';
  const corrected = o.results.filter((r) => r.superseded_by).length;
  return `<div class="nobreak"><h2>${e(o.title)}</h2>
<p class="small">Fecha de la orden: ${e(fmtDate(o.ordered_at))} · Estado: ${e(STATUS[o.status] ?? o.status)}${o.lab_name ? ` · Laboratorio: ${e(o.lab_name)}` : ''}${o.ordered_by_name ? ` · Solicitó: ${e(o.ordered_by_name)}` : ''}</p>
${o.status === 'cancelado' ? `<p class="small"><strong>Orden cancelada:</strong> ${e(o.cancel_reason)}</p>` : ''}</div>
${body}
${corrected ? `<p class="small">Esta orden tiene ${corrected} resultado(s) corregido(s); solo se imprimen los vigentes. El historial completo está en el expediente.</p>` : ''}`;
}

export function labReportHtml(patient: Patient, orders: LabOrder[], clinic: Issuer, notice: string): string {
  const list = orders.filter((o) => o.status !== 'cancelado' || o.results.length > 0).sort((a, b) => b.ordered_at.localeCompare(a.ordered_at));
  const body = `<div class="head">${issuerBlock(clinic)}<div class="doc"><h1>Resultados de laboratorio</h1><div class="small">Impreso: ${e(fmtDateTime(new Date().toISOString()))}</div></div></div>
<div class="box nobreak">${patientBlock(patient)}</div>
${list.length ? list.map(orderHtml).join('') : '<p>Sin órdenes de laboratorio.</p>'}
<p class="small" style="margin-top:12px">Indicadores: Normal dentro del rango de referencia, ↑ alto, ↓ bajo, ‼ crítico, ! alterado, — sin rango para comparar. ${e(notice)}</p>
<div class="sig nobreak">Sello y firma del profesional</div>
<div class="footer">${e(FOOTER_CONF)}</div>`;
  return doc('Resultados de laboratorio', body, '.grid-t th,.grid-t td{border:1px solid #9DB7D8;padding:3px 6px;font-size:11px}.grid-t th{background:#E3EEFB;color:#0B2540}');
}

export interface LabProfessional {
  name: string;
  title: string;
  cedula: string;
  institution: string;
}

/**
 * The sheet the patient takes to a laboratory: the studies asked for (with what each panel includes), the indications and the
 * professional who requests them. It has no results; those are captured when the patient comes back.
 */
export function labOrderSheetHtml(patient: Patient, order: LabOrder, clinic: Issuer, catalog: LabCatalog | null, pro: LabProfessional): string {
  const asked = order.requested.length ? order.requested : [order.title];
  const items = asked
    .map((name) => {
      const panel = catalog?.panels.find((p) => p.name === name);
      const detail = panel ? `<div class="small" style="margin-left:18px">Incluye: ${e(panel.analytes.map((a) => a.name).join(', '))}.</div>` : '';
      return `<li class="nobreak" style="margin:5px 0"><strong>${e(name)}</strong>${detail}</li>`;
    })
    .join('');
  const body = `<div class="head">${issuerBlock(clinic)}<div class="doc"><h1>Orden de laboratorio</h1><div class="small">Fecha: ${e(fmtDate(order.ordered_at))}</div></div></div>
<div class="box nobreak">${patientBlock(patient)}</div>
<h2>Estudios solicitados</h2>
<ul style="margin:6px 0 0;padding-left:18px;font-size:13px;line-height:1.5">${items}</ul>
${order.lab_name ? `<p><span class="k">Laboratorio sugerido</span>${e(order.lab_name)}</p>` : ''}
${order.notes ? `<p><span class="k">Indicaciones para el paciente</span>${e(order.notes)}</p>` : ''}
<p class="small" style="margin-top:12px">Entrega esta orden en el laboratorio y, cuando tengas tus resultados, preséntalos en tu consulta. Si el laboratorio pide ayuno u otra preparación, sigue sus indicaciones.</p>
<div class="box nobreak" style="margin-top:14px"><span class="k">Profesional que solicita</span><strong>${e(pro.name)}</strong>${pro.title ? `<div>${e(pro.title)}</div>` : ''}
${pro.cedula ? `<div class="small">Cédula profesional: <strong>${e(pro.cedula)}</strong>${pro.institution ? ` · Título expedido por ${e(pro.institution)}` : ''}</div>` : ''}</div>
<div class="sig nobreak">Firma y sello del profesional<br>${e(pro.name)}</div>
<div class="footer">${e(FOOTER_CONF)}</div>`;
  return doc('Orden de laboratorio', body, '');
}
