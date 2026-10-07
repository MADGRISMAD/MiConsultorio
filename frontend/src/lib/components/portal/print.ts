import { printHtml } from '$lib/printer/ticket';
import { doc, e, fmtDate, fmtDateTime, multiline } from '$lib/print/base';
import type { PortalPrescription } from '$lib/types/portal';

type ClinicBlock = { name: string; address: string; phone: string };

/** The patient's own copy of a receta. The signed original stays with the clinic. */
export function portalRecetaHtml(rx: PortalPrescription, clinic: ClinicBlock, animal: boolean): string {
  const instr = rx.mode === 'instructions';
  const items = rx.items
    .map((it) => {
      const parts = [it.presentation, it.dose && `Dosis: ${it.dose}`, it.route && `Vía: ${it.route}`, it.frequency && `Frecuencia: ${it.frequency}`, it.duration && `Duración: ${it.duration}`, it.quantity && `Cantidad: ${it.quantity}`]
        .filter(Boolean)
        .map((s) => e(s))
        .join(' &middot; ');
      return `<li class="nobreak"><div><strong>${e(it.medicine)}</strong>${it.brand ? ` (${e(it.brand)})` : ''}</div><div>${parts}</div>${it.notes ? `<div class="small">${multiline(it.notes)}</div>` : ''}</li>`;
    })
    .join('');
  const title = instr ? 'Hoja de indicaciones' : 'Receta';
  const body = `
${rx.voided ? '<div class="watermark">CANCELADA</div>' : ''}
<div class="head"><div class="est"><h1>${e(clinic.name)}</h1><div class="small">${e(clinic.address)}${clinic.phone ? ` &middot; Tel. ${e(clinic.phone)}` : ''}</div></div>
<div class="doc"><h1>${title}</h1><div><strong>Folio ${e(String(rx.folio).padStart(6, '0'))}</strong></div><div class="small">Emitida: ${e(fmtDateTime(rx.issued_at))}</div>
${rx.valid_until ? `<div class="small">Vigente hasta: ${e(fmtDate(rx.valid_until))}</div>` : ''}${animal ? '<div class="small"><strong>Uso veterinario</strong></div>' : ''}</div></div>
<div class="box nobreak"><span class="k">Paciente</span> <strong>${e(rx.patient_name)}</strong></div>
<div class="box nobreak"><span class="k">Profesional que prescribe</span> <strong>${e(rx.author_name)}</strong>${rx.author_title ? ` &mdash; ${e(rx.author_title)}` : ''}<br>
<span class="small">Cédula profesional: <strong>${e(rx.author_license)}</strong>${rx.author_institution ? ` &middot; Título expedido por: ${e(rx.author_institution)}` : ''}</span></div>
${instr ? `<h2>Indicaciones</h2><p style="font-size:13px">${multiline(rx.instructions)}</p>` : `<h2>Rp.</h2><ol style="padding-left:20px;margin:0">${items}</ol>${rx.instructions ? `<h2>Indicaciones generales</h2><p>${multiline(rx.instructions)}</p>` : ''}`}
${rx.next_visit ? `<p><span class="k">Próxima cita</span> ${e(fmtDate(rx.next_visit))}</p>` : ''}
${rx.voided ? `<div class="box"><strong>Receta cancelada</strong>${rx.voided_at ? ` el ${e(fmtDate(rx.voided_at))}` : ''}. No la uses.</div>` : ''}
<div class="footer">Copia para el paciente descargada del portal de ${e(clinic.name)}. Esta copia no lleva la firma autógrafa del profesional; para surtir medicamentos controlados o retenidos presenta la receta firmada que te entregó el consultorio.</div>`;
  return doc(`${title} ${rx.folio}`, body, 'ol li { margin-bottom: 10px; font-size: 13px; } .footer{bottom:-12mm}');
}

export async function printPortalReceta(rx: PortalPrescription, clinic: ClinicBlock, animal: boolean): Promise<void> {
  await printHtml(portalRecetaHtml(rx, clinic, animal));
}

type CarnetRow = { name: string; applied_on: string; next_due: string | null; lot: string; dose: string; administered_by_name: string };

export function portalCarnetHtml(patientName: string, clinicName: string, rows: CarnetRow[]): string {
  const tr = rows
    .map((v) => `<tr><td>${e(v.name)}</td><td>${e(fmtDate(v.applied_on))}</td><td>${e(v.next_due ? fmtDate(v.next_due) : '')}</td><td>${e(v.dose)}</td><td>${e(v.lot)}</td><td>${e(v.administered_by_name)}</td></tr>`)
    .join('');
  const body = `<div class="head"><div class="est"><h1>Carnet de vacunación</h1><div class="small">${e(clinicName)}</div></div><div class="doc"><strong>${e(patientName)}</strong></div></div>
<table class="grid"><thead><tr><th>Aplicación</th><th>Fecha</th><th>Próxima</th><th>Dosis</th><th>Lote</th><th>Aplicó</th></tr></thead><tbody>${tr}</tbody></table>
<div class="footer">Copia para el paciente descargada del portal de ${e(clinicName)}.</div>`;
  return doc('Carnet de vacunación', body, 'table.grid th,table.grid td{border:1px solid #000;padding:4px 6px;text-align:left;font-size:11px}');
}

export async function printPortalCarnet(patientName: string, clinicName: string, rows: CarnetRow[]): Promise<void> {
  await printHtml(portalCarnetHtml(patientName, clinicName, rows));
}
