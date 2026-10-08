import { bodySvg, kindLabel, zoneLabel } from '$lib/components/specialty/bodyGeometry';
import { odontogramSvg, STATES, toothSummary } from '$lib/components/specialty/odontoGeometry';
import type { Issuer, Patient } from '$lib/types';
import type { BodymapData, Consent, NutritionPlanData, OdontogramData, TreatmentPlan, Vaccination, WeightPoint } from '$lib/types/specialty';
import { doc, e, fmtDate, fmtDateTime, FOOTER_CONF, issuerBlock, multiline, patientBlock } from './base';

const KIND_LABEL: Record<string, string> = { vaccine: 'Vacuna', deworming_internal: 'Desparasitación interna', deworming_external: 'Desparasitación externa', other: 'Otro' };
const money = (cents: number) => (cents / 100).toLocaleString('es-MX', { style: 'currency', currency: 'MXN' });

export const CONSENT_LABEL: Record<string, string> = {
  procedimiento: 'Consentimiento informado para procedimiento',
  plan_tratamiento: 'Aceptación de plan de tratamiento',
  privacidad: 'Aviso de privacidad (acuse)',
  telemedicina: 'Consentimiento para telemedicina',
  animal: 'Autorización del propietario (paciente animal)',
  psicologia: 'Consentimiento para atención psicológica'
};

export function carnetHtml(patient: Patient, list: Vaccination[], weights: WeightPoint[], clinic: Issuer): string {
  const animal = patient.subject === 'animal';
  const active = list.filter((v) => !v.voided_at).sort((a, b) => a.applied_on.localeCompare(b.applied_on));
  const group = (kinds: string[]) => active.filter((v) => kinds.includes(v.kind));
  const table = (rows: Vaccination[]) =>
    rows.length === 0
      ? '<p class="small">Sin registros.</p>'
      : `<table class="grid-t"><thead><tr><th>Fecha</th><th>Producto</th><th>Dosis</th><th>Lote</th><th>Próximo refuerzo</th><th>Aplicó</th></tr></thead><tbody>${rows
          .map(
            (v) =>
              `<tr class="nobreak"><td>${e(fmtDate(v.applied_on))}</td><td><strong>${e(v.name)}</strong>${v.notes ? `<div class="small">${e(v.notes)}</div>` : ''}</td><td>${e(v.dose)}</td><td>${e(v.lot)}</td><td>${v.next_due ? e(fmtDate(v.next_due)) : ''}</td><td>${e(v.administered_by_name)}</td></tr>`
          )
          .join('')}</tbody></table>`;
  const wTable = weights.length
    ? `<h2>Historial de peso</h2><table class="grid-t"><thead><tr><th>Fecha</th><th>Peso</th></tr></thead><tbody>${weights.map((w) => `<tr><td>${e(fmtDate(w.date))}</td><td>${w.weight_kg} kg</td></tr>`).join('')}</tbody></table>`
    : '';
  const body = `<div class="head">${issuerBlock(clinic)}<div class="doc"><h1>Carnet de ${animal ? 'vacunación y desparasitación' : 'vacunación'}</h1><div class="small">Impreso: ${e(fmtDateTime(new Date().toISOString()))}</div></div></div>
<div class="box nobreak">${patientBlock(patient)}</div>
<h2>Vacunas</h2>${table(group(['vaccine', 'other']))}
${animal ? `<h2>Desparasitación interna</h2>${table(group(['deworming_internal']))}<h2>Desparasitación externa</h2>${table(group(['deworming_external']))}` : ''}
${wTable}
<div class="sig nobreak">Sello y firma del ${animal ? 'Médico Veterinario' : 'profesional'}</div>
<div class="footer">${e(FOOTER_CONF)}</div>`;
  return doc('Carnet de vacunación', body, '.grid-t th,.grid-t td{border:1px solid #000;padding:3px 6px;font-size:11px}.grid-t th{background:#eee}');
}

function legendHtml(): string {
  return `<div class="small" style="margin-top:6px">${STATES.map((s) => `<span style="display:inline-block;margin-right:10px"><span style="display:inline-block;width:10px;height:10px;background:${s.color};vertical-align:-1px;margin-right:3px"></span>${e(s.label)}</span>`).join('')}</div>`;
}

export function chartHtml(
  patient: Patient,
  kind: 'odontogram' | 'bodymap',
  chart: { data: OdontogramData | BodymapData; note: string; at: string; by: string },
  clinic: Issuer,
  highlight: Set<number> = new Set()
): string {
  let content: string;
  let title: string;
  if (kind === 'odontogram') {
    const d = chart.data as OdontogramData;
    title = 'Odontograma';
    const rows = Object.keys(d.teeth)
      .sort((a, b) => Number(a) - Number(b))
      .map((k) => `<tr class="nobreak"><td><strong>${e(k)}</strong></td><td>${e(toothSummary(d.teeth[k]))}${d.teeth[k].note ? `<div class="small">${e(d.teeth[k].note)}</div>` : ''}</td></tr>`)
      .join('');
    content = `<div style="text-align:center">${odontogramSvg(d, highlight)}</div>${legendHtml()}
<h2>Detalle por pieza</h2>${rows ? `<table class="grid-t"><thead><tr><th>Pieza</th><th>Hallazgos</th></tr></thead><tbody>${rows}</tbody></table>` : '<p class="small">Sin hallazgos registrados.</p>'}`;
  } else {
    const d = chart.data as BodymapData;
    title = 'Esquema corporal';
    const rows = [...d.zones]
      .sort((a, b) => b.intensity - a.intensity)
      .map((f) => `<tr class="nobreak"><td>${e(zoneLabel(f.view, f.zone))} (${f.view === 'front' ? 'frente' : 'espalda'})</td><td>${e(kindLabel(f.kind))}</td><td>${f.intensity}/10</td><td>${e(f.note)}</td></tr>`)
      .join('');
    content = `<div style="display:flex;justify-content:center;gap:30px"><div class="center"><div class="small">Frente</div>${bodySvg('front', d.zones)}</div><div class="center"><div class="small">Espalda</div>${bodySvg('back', d.zones)}</div></div>
<h2>Hallazgos</h2>${rows ? `<table class="grid-t"><thead><tr><th>Zona</th><th>Tipo</th><th>Intensidad</th><th>Nota</th></tr></thead><tbody>${rows}</tbody></table>` : '<p class="small">Sin hallazgos registrados.</p>'}`;
  }
  const body = `<div class="head">${issuerBlock(clinic)}<div class="doc"><h1>${title}</h1><div class="small">${e(fmtDateTime(chart.at))}</div>${chart.by ? `<div class="small">Registró: ${e(chart.by)}</div>` : ''}</div></div>
<div class="box nobreak">${patientBlock(patient)}</div>
${chart.note ? `<p><span class="k">Nota</span>${multiline(chart.note)}</p>` : ''}
${content}
<div class="sig nobreak">Firma del profesional</div>
<div class="footer">${e(FOOTER_CONF)}</div>`;
  return doc(title, body, '.grid-t th,.grid-t td{border:1px solid #000;padding:3px 6px;font-size:11px}.grid-t th{background:#eee}');
}

export function nutritionPlanHtml(patient: Patient, plan: { data: NutritionPlanData; note: string; at: string; by: string }, clinic: Issuer): string {
  const d = plan.data;
  const grams = (pct: number, per: number) => (d.kcal && pct ? Math.round((d.kcal * pct) / 100 / per) : 0);
  const macro = (label: string, pct: number, per: number) => (pct ? `<td><strong>${e(label)}</strong><br>${pct} % · ${grams(pct, per)} g</td>` : '');
  const meals = d.meals
    .map((m) => `<tr class="nobreak"><td><strong>${e(m.name)}</strong>${m.time ? `<div class="small">${e(m.time)}</div>` : ''}</td><td>${multiline(m.items)}</td><td style="text-align:right">${m.kcal ? `${m.kcal} kcal` : ''}</td></tr>`)
    .join('');
  const block = (title: string, text: string) => (text.trim() ? `<h2>${e(title)}</h2><p>${multiline(text)}</p>` : '');
  const body = `<div class="head">${issuerBlock(clinic)}<div class="doc"><h1>Plan nutricional</h1><div class="small">${e(fmtDateTime(plan.at))}</div>${plan.by ? `<div class="small">Elaboró: ${e(plan.by)}</div>` : ''}</div></div>
<div class="box nobreak">${patientBlock(patient)}</div>
${d.goal ? `<p><span class="k">Objetivo</span><strong>${e(d.goal)}</strong></p>` : ''}
${plan.note ? `<p><span class="k">Nota</span>${multiline(plan.note)}</p>` : ''}
${d.kcal ? `<table class="grid-t nobreak"><tbody><tr><td><strong>Energía</strong><br>${d.kcal} kcal al día</td>${macro('Proteínas', d.protein_pct, 4)}${macro('Carbohidratos', d.carb_pct, 4)}${macro('Grasas', d.fat_pct, 9)}${d.water_liters ? `<td><strong>Agua</strong><br>${d.water_liters} L al día</td>` : ''}</tr></tbody></table>` : ''}
${meals ? `<h2>Distribución de comidas</h2><table class="grid-t"><thead><tr><th>Comida</th><th>Alimentos</th><th>Energía</th></tr></thead><tbody>${meals}</tbody></table>` : ''}
${block('Recomendaciones', d.recommendations)}${block('Alimentos o hábitos a evitar', d.avoid)}${block('Suplementos', d.supplements)}
${d.follow_up_days ? `<p><span class="k">Siguiente cita</span>En ${d.follow_up_days} días aproximadamente.</p>` : ''}
${d.basis ? `<p class="small">${e(d.basis)}</p>` : ''}
<div class="sig nobreak">Firma del nutriólogo</div>
<div class="footer">${e(FOOTER_CONF)}</div>`;
  return doc('Plan nutricional', body, '.grid-t th,.grid-t td{border:1px solid #000;padding:4px 8px;font-size:11px;vertical-align:top}.grid-t th{background:#eee}');
}

const STATUS: Record<string, string> = { draft: 'Borrador', proposed: 'Propuesto', accepted: 'Aceptado', in_progress: 'En curso', completed: 'Completado', cancelled: 'Cancelado' };

export function planHtml(plan: TreatmentPlan, patient: Patient, clinic: Issuer, consent?: Consent | null): string {
  const phases = [...new Set(plan.items.map((i) => i.phase))].sort((a, b) => a - b);
  const money2 = money;
  const phaseHtml = phases
    .map((ph) => {
      const items = plan.items.filter((i) => i.phase === ph);
      const sub = items.filter((i) => i.status !== 'cancelled').reduce((s, i) => s + i.total_cents, 0);
      return `<h2>Fase ${ph}</h2><table class="grid-t"><thead><tr><th>Concepto</th><th>Pieza / zona</th><th>Cant.</th><th>Precio</th><th>Importe</th><th>Estado</th></tr></thead><tbody>${items
        .map(
          (i) =>
            `<tr class="nobreak ${i.status === 'cancelled' ? 'strike' : ''}"><td>${e(i.description)}</td><td>${e(i.tooth)}</td><td>${i.qty}</td><td>${money2(i.unit_price_cents)}</td><td>${money2(i.total_cents)}</td><td>${i.status === 'done' ? `Realizado ${e(fmtDate(i.done_at))}` : i.status === 'cancelled' ? 'Cancelado' : 'Pendiente'}</td></tr>`
        )
        .join('')}<tr><td colspan="4" style="text-align:right"><strong>Subtotal de la fase</strong></td><td colspan="2"><strong>${money2(sub)}</strong></td></tr></tbody></table>`;
    })
    .join('');
  const sig =
    consent && consent.signature_png
      ? `<div class="box nobreak"><span class="k">Aceptación firmada</span>El ${e(fmtDateTime(consent.signed_at))}, <strong>${e(consent.signer_name)}</strong> (${e(consent.signer_role)}) aceptó el plan.<br>
<img src="${e(consent.signature_png)}" alt="Firma" style="height:70px;margin-top:4px"><div class="small">Huella digital del documento (SHA-256): ${e(consent.content_sha256)}</div></div>`
      : `<div class="sigs nobreak"><div class="sig">Firma de aceptación del paciente o responsable</div><div class="sig">Firma del profesional</div></div>`;
  const body = `<div class="head">${issuerBlock(clinic)}<div class="doc"><h1>${plan.status === 'draft' || plan.status === 'proposed' ? 'Presupuesto' : 'Plan'} de tratamiento</h1><div class="small">Versión ${plan.version} · ${e(STATUS[plan.status])}</div><div class="small">${e(fmtDate(plan.created_at))}</div></div></div>
<div class="box nobreak">${patientBlock(patient)}</div>
<p><span class="k">Plan</span><strong>${e(plan.title)}</strong></p>${plan.notes ? `<p>${multiline(plan.notes)}</p>` : ''}
${phaseHtml}
<div class="box nobreak" style="text-align:right"><span class="k">Total del plan (IVA incluido cuando aplica)</span><strong style="font-size:16px">${money2(plan.total_cents)}</strong><div class="small">Realizado: ${money2(plan.done_cents)} · Pendiente: ${money2(plan.pending_cents)}</div></div>
<p class="small">Los precios pueden cambiar si el diagnóstico cambia; cualquier concepto nuevo requiere tu autorización.</p>
${sig}
<div class="footer">${e(FOOTER_CONF)}</div>`;
  return doc('Plan de tratamiento', body, '.grid-t th,.grid-t td{border:1px solid #000;padding:3px 6px;font-size:11px}.grid-t th{background:#eee}.strike{text-decoration:line-through;color:#555}');
}

export function signedConsentHtml(c: Consent, patient: Patient | null, patientName: string, clinic: Issuer): string {
  const body = `<div class="head">${issuerBlock(clinic)}<div class="doc"><h1>${e(CONSENT_LABEL[c.kind] ?? 'Consentimiento informado')}</h1><div class="small">Firmado: ${e(fmtDateTime(c.signed_at))}</div></div></div>
<div class="box nobreak">${patient ? patientBlock(patient) : `<strong>${e(patientName)}</strong>`}</div>
<div style="font-size:12.5px;line-height:1.6">${multiline(c.text_snapshot ?? '')}</div>
<div class="box nobreak" style="margin-top:16px"><span class="k">Firma</span><img src="${e(c.signature_png ?? '')}" alt="Firma de ${e(c.signer_name)}" style="height:80px;display:block;margin:2px 0">
<strong>${e(c.signer_name)}</strong> (${e(c.signer_role)})
${c.witness1 || c.witness2 ? `<div class="small">Testigos: ${[c.witness1, c.witness2].filter(Boolean).map(e).join(' y ')}</div>` : ''}
<div class="small">Registró: ${e(c.registered_by_name)} · Huella digital (SHA-256): <span style="word-break:break-all">${e(c.content_sha256)}</span></div></div>
<div class="footer">${e(FOOTER_CONF)}</div>`;
  return doc(CONSENT_LABEL[c.kind] ?? 'Consentimiento', body);
}
