import { FLAG_LABEL, FLAG_MARK, rangeText } from '$lib/components/lab/labUtil';
import type { Issuer, Patient } from '$lib/types';
import type { LabOrder, LabResult } from '$lib/types/lab';
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
        return `<tr class="nobreak"><td>${e(r.analyte)}${corr}</td><td><strong>${e(valueText(r))}</strong></td><td>${e(range)}${e(src)}</td><td>${r.flag === 'normal' || r.flag === 'na' ? '' : `<strong>${e(FLAG_MARK[r.flag])} ${e(FLAG_LABEL[r.flag])}</strong>`}</td></tr>`;
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
<p class="small" style="margin-top:12px">Indicadores: ↑ alto, ↓ bajo, ‼ crítico, ! alterado. ${e(notice)}</p>
<div class="sig nobreak">Sello y firma del profesional</div>
<div class="footer">${e(FOOTER_CONF)}</div>`;
  return doc('Resultados de laboratorio', body, '.grid-t th,.grid-t td{border:1px solid #9DB7D8;padding:3px 6px;font-size:11px}.grid-t th{background:#E3EEFB;color:#0B2540}');
}
