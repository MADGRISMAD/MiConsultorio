import { bodySvg, kindLabel, zoneLabel } from '$lib/components/specialty/bodyGeometry';
import { odontogramSvg, STATES, toothSummary } from '$lib/components/specialty/odontoGeometry';
import type { Issuer, Patient } from '$lib/types';
import type { BodymapData, Consent, NutritionPlanData, OdontogramData, TreatmentPlan, Vaccination, WeightPoint } from '$lib/types/specialty';
import { doc, e, fmtDate, fmtDateTime, FOOTER_CONF, issuerBlock, multiline, patientBlock } from './base';

const KIND_LABEL: Record<string, string> = { vaccine: 'Vacuna', deworming_internal: 'Desparasitación interna', deworming_external: 'Desparasitación externa', other: 'Otro' };
import { moneyCents as money } from '$lib/format';

export const CONSENT_LABEL: Record<string, string> = {
  procedimiento: 'Consentimiento informado para procedimiento',
  plan_tratamiento: 'Aceptación de plan de tratamiento',
  privacidad: 'Aviso de privacidad (acuse)',
  telemedicina: 'Consentimiento para telemedicina',
  animal: 'Autorización del propietario (paciente animal)',
  psicologia: 'Consentimiento para atención psicológica'
};

/** `patientCopy` replaces the signature line and the footer with the note of the patient's own copy (portal). */
export function carnetHtml(patient: Patient, list: Vaccination[], weights: WeightPoint[], clinic: Issuer, patientCopy?: string): string {
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
${patientCopy ? '' : `<div class="sig nobreak">Sello y firma del ${animal ? 'Médico Veterinario' : 'profesional'}</div>`}
<div class="footer">${e(patientCopy ?? FOOTER_CONF)}</div>`;
  return doc('Carnet de vacunación', body, '.grid-t th,.grid-t td{border:1px solid #9DB7D8;padding:3px 6px;font-size:11px}.grid-t th{background:#E3EEFB;color:#0B2540}');
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
  return doc(title, body, '.grid-t th,.grid-t td{border:1px solid #9DB7D8;padding:3px 6px;font-size:11px}.grid-t th{background:#E3EEFB;color:#0B2540}');
}

/** Shrinks the menu's text until page 1 holds it all, then stretches the table to fill what is left of the sheet. */
const FIT_MENU_SCRIPT = `<script>(function(){var m=document.querySelector('.menu');if(!m)return;var max=246/25.4*96,fs=10;m.style.width='195mm';m.style.setProperty('--fs',fs+'px');while(m.offsetHeight>max&&fs>5.5){fs-=0.25;m.style.setProperty('--fs',fs+'px');}var t=m.querySelector('.week');if(t&&m.offsetHeight<max){t.style.height=(t.offsetHeight+max-m.offsetHeight-6)+'px';}})();</script>`;

export function nutritionPlanHtml(patient: Patient, plan: { data: NutritionPlanData; note: string; at: string; by: string; author?: { title: string; cedula: string; phone: string; email: string } }, clinic: Issuer): string {
  const d = plan.data;
  const grams = (pct: number, per: number) => (d.kcal && pct ? Math.round((d.kcal * pct) / 100 / per) : 0);
  const macro = (label: string, pct: number, per: number) => (pct ? `<td><strong>${e(label)}</strong><br>${pct} % · ${grams(pct, per)} g</td>` : '');
  const mealRows = (list: NutritionPlanData['meals']) =>
    list
      .map((m) => `<tr class="nobreak"><td><strong>${e(m.name)}</strong>${m.time ? `<div class="small">${e(m.time)}</div>` : ''}</td><td>${multiline(m.items)}</td><td style="text-align:right">${m.kcal ? `${m.kcal} kcal` : ''}</td></tr>`)
      .join('');
  const days = d.days?.length ? d.days : d.meals?.length ? [{ name: 'Día tipo', meals: d.meals }] : [];
  // weekly calendar: one row per day, one column per meal (by position, like the screen). Compact so a week fits a portrait page.
  const cols = (days[0]?.meals ?? []).map((m) => m.name);
  const kcalOf = (list: NutritionPlanData['meals']) => list.reduce((s, m) => s + (m.kcal || 0), 0);
  const meals = cols.length
    ? `<table class="grid-t week"><thead><tr><th class="day"></th>${cols.map((c) => `<th>${e(c)}</th>`).join('')}</tr></thead><tbody>${days
        .map(
          (day) =>
            `<tr><th class="day">${e(day.name)}${kcalOf(day.meals) ? `<div class="k">${kcalOf(day.meals)} kcal</div>` : ''}</th>${cols
              .map((_, i) => `<td>${day.meals[i] ? multiline(day.meals[i].items) + (day.meals[i].kcal ? `<span class="k"> · ${day.meals[i].kcal} kcal</span>` : '') : ''}</td>`)
              .join('')}</tr>`
        )
        .join('')}</tbody></table>`
    : '';
  const block = (title: string, text: string) => (text.trim() ? `<h2>${e(title)}</h2><p>${multiline(text)}</p>` : '');
  // Page 1 is the menu alone, filling the sheet and signed right under it (the patient often keeps only that sheet).
  // Everything else (advice, foods to avoid, supplements, next visit) goes on page 2.
  const extras = [block('Alimentos que no le gustan (se evitaron en el menú)', d.dislikes ?? ''), block('Recomendaciones', d.recommendations), block('Alimentos o hábitos a evitar', d.avoid), block('Suplementos', d.supplements)].join('');
  // who attended: name, profession, cédula and contact, in the same data box as the patient
  const au = plan.author;
  const proRow = (k: string, v: string) => (v ? `<div><span class="k">${e(k)}</span>${e(v)}</div>` : '');
  const proBlock = plan.by || au
    ? `<div class="grid pro">${proRow('Atendió', plan.by)}${proRow('Profesión', au?.title ?? '')}${proRow('Cédula profesional', au?.cedula ?? '')}${proRow('Contacto', [au?.phone, au?.email].filter(Boolean).join(' · '))}</div>`
    : '';
  const hasPage2 = !!(extras || d.follow_up_days || plan.note);
  const body = `<section class="menu"><div class="head">${issuerBlock(clinic)}<div class="doc"><h1>Plan nutricional</h1><div class="small">${e(fmtDateTime(plan.at))}</div>${plan.by ? `<div class="small">Elaboró: ${e(plan.by)}</div>` : ''}</div></div>
<div class="box nobreak pair">${patientBlock(patient)}${proBlock}</div>
${d.goal ? `<p><span class="k">Objetivo</span><strong>${e(d.goal)}</strong></p>` : ''}
${d.kcal ? `<table class="grid-t nobreak"><tbody><tr><td><strong>Energía</strong><br>${d.kcal} kcal al día</td>${macro('Proteínas', d.protein_pct, 4)}${macro('Carbohidratos', d.carb_pct, 4)}${macro('Grasas', d.fat_pct, 9)}${d.water_liters ? `<td><strong>Agua</strong><br>${d.water_liters} L al día</td>` : ''}</tr></tbody></table>` : ''}
${meals ? `<h2 style="margin-top:12px">${days.length > 1 ? 'Alimentación semanal' : 'Menú del día'}</h2>${meals}` : ''}
<div class="sig nobreak">Firma del nutriólogo${plan.by ? ` · ${e(plan.by)}` : ''}</div></section>
${
    hasPage2
      ? `<section class="more"><div class="head"><div><strong>${e(patient.names)} ${e(patient.last_names ?? '')}</strong><div class="small">Plan nutricional · ${e(fmtDate(plan.at))}</div></div><div class="doc small">${e(clinic.name ?? '')}</div></div>
${plan.note ? `<p><span class="k">Nota</span>${multiline(plan.note)}</p>` : ''}${extras}
${d.follow_up_days ? `<p><span class="k">Siguiente cita</span>En ${d.follow_up_days} días aproximadamente.</p>` : ''}
<div class="small" style="margin-top:14px;text-align:center;color:#333">${e(FOOTER_CONF)}</div></section>`
      : `<div class="small" style="margin-top:8px;text-align:center;color:#333">${e(FOOTER_CONF)}</div>`
  }${FIT_MENU_SCRIPT}`;
  return doc(
    'Plan nutricional',
    body,
    'h2{margin:10px 0 4px;break-after:avoid;page-break-after:avoid}p{orphans:2;widows:2}' +
      '.menu{break-after:page;page-break-after:always}.more{padding-top:2mm}.more p{font-size:12.5px;line-height:1.5}.more h2{font-size:14px}' +
      '.menu .head{padding:7px 12px 15px;margin-bottom:5px}.menu .brand .logo,.menu .brand .logo-svg{width:42px;height:42px}.menu h1{font-size:16px}.menu .box{padding:4px 8px;margin:4px 0}.menu .grid{grid-template-columns:1fr 1fr;gap:2px 10px;font-size:10px}.pair{display:grid;grid-template-columns:1fr 1fr;gap:0 12px}.pair .pro{border-top:0;border-left:1px solid #999;margin:0;padding:0 0 0 12px}.menu .grid .k{font-size:7.5px}.menu p{margin:0 0 3px}.menu h2{margin:6px 0 3px;font-size:12px}.menu .sig{margin-top:8mm}.menu{--fs:10px}.grid-t th,.grid-t td{border:1px solid #9DB7D8;padding:4px 5px;font-size:var(--fs);line-height:1.25;vertical-align:top}.grid-t th{background:#E3EEFB;color:#0B2540}' +
      '.week{table-layout:fixed}.week thead{display:table-header-group}.week tr{break-inside:avoid;page-break-inside:avoid}' +
      '.week thead{height:8mm}.week th.day{width:10%;text-align:left;font-size:10.5px}.week thead th{text-align:center;text-transform:uppercase;letter-spacing:.03em}' +
      '.week .k{color:#444;font-size:8.4px;font-weight:400;display:inline}.week th.day .k{display:block;margin-top:2px}'
  );
}

const STATUS: Record<string, string> = { draft: 'Borrador', proposed: 'Propuesto', accepted: 'Aceptado', in_progress: 'En curso', completed: 'Completado', cancelled: 'Cancelado' };

export function planHtml(plan: TreatmentPlan, patient: Patient, clinic: Issuer, consent?: Consent | null): string {
  const phases = [...new Set(plan.items.map((i) => i.phase))].sort((a, b) => a - b);
  const phaseHtml = phases
    .map((ph) => {
      const items = plan.items.filter((i) => i.phase === ph);
      const sub = items.filter((i) => i.status !== 'cancelled').reduce((s, i) => s + i.total_cents, 0);
      return `<h2>Fase ${ph}</h2><table class="grid-t"><thead><tr><th>Concepto</th><th>Pieza / zona</th><th>Cant.</th><th>Precio</th><th>Importe</th><th>Estado</th></tr></thead><tbody>${items
        .map(
          (i) =>
            `<tr class="nobreak ${i.status === 'cancelled' ? 'strike' : ''}"><td>${e(i.description)}</td><td>${e(i.tooth)}</td><td>${i.qty}</td><td>${money(i.unit_price_cents)}</td><td>${money(i.total_cents)}</td><td>${i.status === 'done' ? `Realizado ${e(fmtDate(i.done_at))}` : i.status === 'cancelled' ? 'Cancelado' : 'Pendiente'}</td></tr>`
        )
        .join('')}<tr><td colspan="4" style="text-align:right"><strong>Subtotal de la fase</strong></td><td colspan="2"><strong>${money(sub)}</strong></td></tr></tbody></table>`;
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
<div class="box nobreak" style="text-align:right"><span class="k">Total del plan (IVA incluido cuando aplica)</span><strong style="font-size:16px">${money(plan.total_cents)}</strong><div class="small">Realizado: ${money(plan.done_cents)} · Pendiente: ${money(plan.pending_cents)}</div></div>
<p class="small">Los precios pueden cambiar si el diagnóstico cambia; cualquier concepto nuevo requiere tu autorización.</p>
${sig}
<div class="footer">${e(FOOTER_CONF)}</div>`;
  return doc('Plan de tratamiento', body, '.grid-t th,.grid-t td{border:1px solid #9DB7D8;padding:3px 6px;font-size:11px}.grid-t th{background:#E3EEFB;color:#0B2540}.strike{text-decoration:line-through;color:#555}');
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
