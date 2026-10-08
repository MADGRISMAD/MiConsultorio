import { ENCOUNTER_KINDS, type Encounter, type FieldDef, type PatientRecord, type PatientSchema } from '$lib/types';
import { doc, e, FOOTER_CONF, fieldValue, fmtDate, fmtDateTime, issuerBlock, multiline, patientBlock, fullName } from './base';

function section(label: string, text: string): string {
  return text ? `<p><span class="k">${e(label)}</span>${multiline(text)}</p>` : '';
}

function measuresHtml(defs: FieldDef[], values: Encounter['measures']): string {
  const rows = defs.map((d) => ({ d, v: fieldValue(d, values) })).filter((r) => r.v);
  if (!rows.length) return '';
  return `<p><span class="k">Signos vitales / mediciones</span>${rows.map((r) => `${e(r.d.label)}: <strong>${e(r.v)}</strong>`).join(' &nbsp;&middot;&nbsp; ')}</p>`;
}

function entryHtml(en: Encounter, defs: FieldDef[], original?: Encounter, adds = false): string {
  const head = `<div style="display:flex;justify-content:space-between;gap:10px"><h3>${e(ENCOUNTER_KINDS[en.kind] ?? en.kind)}${original ? ` (corrige nota del ${e(fmtDateTime(original.occurred_at))})` : ''}</h3><span class="small">${e(fmtDateTime(en.occurred_at))}</span></div>
<div class="small muted">${e(en.author_name)}${en.author_role ? `, ${e(en.author_role)}` : ''}${en.author_license ? ` &middot; Céd. prof. ${e(en.author_license)}` : ''}</div>`;
  if (en.hidden) {
    return `<div class="entry${adds ? ' add' : ''}">${head}<p class="small"><em>Nota privada de ${e(en.author_name)}: no incluida en esta copia.</em></p></div>`;
  }
  const codes = (en.diagnosis_codes ?? []).map((c) => `<span class="chip">${e(c)}</span>`).join('');
  return `<div class="entry${adds ? ' add' : ''}">${head}
${en.private ? '<div class="small"><strong>Nota privada</strong></div>' : ''}
${section(en.kind === 'adenda' ? 'Adenda (qué se corrige)' : 'Motivo', en.reason)}
${section('Lo que cuenta el paciente', en.subjective)}
${measuresHtml(defs, en.measures)}
${section('Exploración', en.exam)}
${section('Diagnóstico', en.assessment)}${codes ? `<p><span class="k">CIE-10</span>${codes}</p>` : ''}
${section('Plan e indicaciones', en.plan)}
${section('Notas', en.notes)}
<div class="sig">Firma de ${e(en.author_name)}</div></div>`;
}

export function expedienteHtml(rec: PatientRecord, schema: PatientSchema): string {
  const { patient: p, clinic } = rec;
  const subj = p.subject;
  const profile = schema.profile[subj] ?? [];
  const measureDefs = (schema.measures_all ?? schema.measures)[subj] ?? [];

  const groups: { name: string; rows: { label: string; v: string }[] }[] = [];
  for (const f of profile) {
    const v = fieldValue(f, p.profile);
    if (!v) continue;
    let g = groups.find((x) => x.name === f.group);
    if (!g) groups.push((g = { name: f.group, rows: [] }));
    g.rows.push({ label: f.label, v });
  }
  const antecedentes = groups.length
    ? groups.map((g) => `<div class="nobreak"><h3 style="margin:8px 0 3px">${e(g.name)}</h3><div class="grid">${g.rows.map((r) => `<div><span class="k">${e(r.label)}</span>${e(r.v)}</div>`).join('')}</div></div>`).join('')
    : '<p class="small">Sin antecedentes capturados.</p>';

  const sorted = [...rec.encounters].sort((a, b) => a.occurred_at.localeCompare(b.occurred_at));
  const byId = new Map(sorted.map((x) => [x.id, x]));
  const roots = sorted.filter((x) => !x.addendum_of || !byId.has(x.addendum_of));
  const entries = roots
    .map((r) => entryHtml(r, measureDefs) + sorted.filter((a) => a.addendum_of === r.id).map((a) => entryHtml(a, measureDefs, r, true)).join(''))
    .join('');

  const rxs = [...rec.prescriptions].sort((a, b) => a.issued_at.localeCompare(b.issued_at));
  const rxHtml = rxs.length
    ? `<table><thead><tr><th>Folio</th><th>Fecha</th><th>Resumen</th><th>Estado</th></tr></thead><tbody>${rxs
        .map((r) => {
          const sum = r.mode === 'instructions' ? r.instructions : r.items.map((i) => i.medicine).join(', ');
          return `<tr class="nobreak"><td>${e(String(r.folio).padStart(6, '0'))}</td><td>${e(fmtDate(r.issued_at))}</td><td>${e(sum.length > 160 ? sum.slice(0, 160) + '…' : sum)}</td><td>${r.voided_at ? `<strong>CANCELADA</strong> (${e(fmtDate(r.voided_at))}${r.void_reason ? `: ${e(r.void_reason)}` : ''})` : 'Vigente/emitida'}</td></tr>`;
        })
        .join('')}</tbody></table>`
    : '<p class="small">Sin recetas registradas.</p>';

  const privacy = p.privacy_notice_at
    ? `Aviso de privacidad registrado el ${e(fmtDate(p.privacy_notice_at))}${p.privacy_notice_by ? ` por ${e(p.privacy_notice_by)}` : ''}.`
    : 'Aviso de privacidad: <strong>pendiente de registrar</strong>.';

  const body = `
<div class="head">${issuerBlock(clinic)}<div class="doc"><h1>EXPEDIENTE CLÍNICO</h1><div><strong>No. ${e(String(p.file_number))}</strong></div>
<div class="small">Generado: ${e(fmtDateTime(rec.generated_at))}</div><div class="small">Impreso por: ${e(rec.printed_by)}</div></div></div>
<h2>Ficha de identificación</h2>
<div class="box">${patientBlock(p)}${p.birth_date ? `<div class="small">Fecha de nacimiento: ${e(fmtDate(p.birth_date))}</div>` : ''}
<div class="small">${[p.email && `Correo: ${p.email}`, p.address && `Domicilio: ${p.address}`, p.guardian_phone && p.subject === 'person' && `Tel. responsable: ${p.guardian_phone}`].filter(Boolean).map((s) => e(s)).join(' &middot; ')}</div></div>
${p.archived_at ? `<p class="small"><strong>Expediente archivado</strong> el ${e(fmtDate(p.archived_at))}${p.archive_reason ? `: ${e(p.archive_reason)}` : ''}</p>` : ''}
<h2>Antecedentes</h2>${antecedentes}
<h2>Bitácora de atención</h2>${entries || '<p class="small">Sin notas registradas.</p>'}
<h2>Recetas</h2>${rxHtml}
<h2>Cierre</h2><p>${privacy}</p>
<p class="small">Expediente de ${e(fullName(p))}. Cada nota debe ser firmada de forma autógrafa por quien la elaboró en la copia impresa.</p>
<div class="footer">${e(FOOTER_CONF)}</div>`;
  return doc(`Expediente ${p.file_number}`, body);
}
