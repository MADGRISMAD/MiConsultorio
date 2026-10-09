import type { Issuer, Patient, Prescription } from '$lib/types';
import { doc, e, fmtDate, fmtDateTime, issuerBlock, multiline, patientBlock } from './base';

/** `verify` carries the QR (a data URL generated in the browser) and the address it points to. */
function verifyHost(url: string): string {
  try {
    return `${new URL(url).host}/verificar`;
  } catch {
    return url;
  }
}

export function recetaHtml(rx: Prescription, patient: Patient, clinic: Issuer, verify?: { url: string; qr: string }): string {
  const instr = rx.mode === 'instructions';
  const animal = patient.subject === 'animal';
  const voided = !!rx.voided_at;
  const title = instr ? 'Hoja de indicaciones' : 'Receta';
  const items = rx.items ?? [];
  const retained = !instr && items.some((i) => i.control === 'Antibiótico' || i.control === 'Fracción III');

  const itemsHtml = items
    .map((it, n) => {
      const parts = [it.presentation, it.dose && `Dosis: ${it.dose}`, it.route && `Vía: ${it.route}`, it.frequency && `Cada/Frecuencia: ${it.frequency}`, it.duration && `Duración: ${it.duration}`, it.quantity && `Cantidad: ${it.quantity}`]
        .filter(Boolean)
        .map((s) => e(s))
        .join(' &middot; ');
      const ret = it.control === 'Antibiótico' || it.control === 'Fracción III';
      return `<li class="nobreak"><div><strong>${e(it.medicine)}</strong>${it.brand ? ` (${e(it.brand)})` : ''}</div>
<div>${parts}</div>${it.notes ? `<div class="small">${multiline(it.notes)}</div>` : ''}
${ret ? `<div class="small"><strong>Receta retenida por la farmacia</strong> (${e(it.control)})</div>` : ''}</li>`;
    })
    .join('');

  const body = `
${voided ? '<div class="watermark">CANCELADA</div>' : ''}
<div class="head">${issuerBlock(clinic)}<div class="doc"><h1>${title}</h1><div><strong>Folio ${e(String(rx.folio).padStart(6, '0'))}</strong></div>
<div class="small">Emitida: ${e(fmtDateTime(rx.issued_at))}</div>${rx.valid_until ? `<div class="small">Vigente hasta: ${e(fmtDate(rx.valid_until))}</div>` : ''}
${animal ? '<div class="small"><strong>Uso veterinario</strong></div>' : ''}</div></div>

<div class="box nobreak pair"><div>${patientBlock(patient)}</div>
<div class="pro"><span class="k">Profesional que prescribe</span>
<strong>${e(rx.author_name)}</strong>${rx.author_title ? `<div>${e(rx.author_title)}</div>` : ''}
<div class="small">Cédula profesional: <strong>${e(rx.author_license)}</strong></div>
${rx.author_institution ? `<div class="small">Título expedido por: ${e(rx.author_institution)}</div>` : ''}${rx.author_specialty_license ? `<div class="small">Cédula de especialidad: ${e(rx.author_specialty_license)}</div>` : ''}</div></div>
${rx.diagnosis ? `<p><span class="k">Diagnóstico</span>${multiline(rx.diagnosis)}</p>` : ''}

${instr ? `<h2>Indicaciones</h2><p style="font-size:13px">${multiline(rx.instructions)}</p>` : `<h2>Rp.</h2><ol style="padding-left:20px;margin:0">${itemsHtml}</ol>
${rx.instructions ? `<h2>Indicaciones generales</h2><p>${multiline(rx.instructions)}</p>` : ''}`}
${retained ? '<p class="small"><strong>Receta retenida por la farmacia</strong> para los medicamentos señalados.</p>' : ''}
${rx.next_visit ? `<p><span class="k">Próxima cita</span>${e(fmtDate(rx.next_visit))}</p>` : ''}
${voided ? `<div class="box"><strong>Receta cancelada</strong> el ${e(fmtDateTime(rx.voided_at))}${rx.voided_by ? ` por ${e(rx.voided_by)}` : ''}.${rx.void_reason ? ` Motivo: ${e(rx.void_reason)}` : ''}</div>` : ''}

${verify ? `<div class="verify nobreak"><img src="${e(verify.qr)}" alt="Código QR de verificación" width="84" height="84"><div class="small"><strong>Verifica esta receta en ${e(verifyHost(verify.url))}</strong><br>Escanea el código o abre la dirección: confirma que el folio, el profesional y la vigencia son auténticos. No muestra medicamentos ni diagnóstico.<br>${e(verify.url)}</div></div>` : ''}
<div class="sig nobreak">Firma autógrafa del ${animal ? 'Médico Veterinario' : 'médico / profesional'}<br>${e(rx.author_name)}</div>
<div class="footer">${instr ? 'Hoja de indicaciones' : 'Receta'} emitida con Caresia.${rx.valid_until ? ` Vigente hasta el ${e(fmtDate(rx.valid_until))}.` : ''} Caresia no firma: la firma autógrafa del profesional da validez al documento.</div>`;
  return doc(`${title} ${rx.folio}`, body, '.pair{display:grid;grid-template-columns:1.1fr 1fr;gap:0 14px}.pair .pro{border-left:1px solid #999;padding-left:12px}ol li { margin-bottom: 10px; font-size: 13px; } .footer{bottom:-12mm} .verify{display:flex;gap:10px;align-items:center;margin-top:18px} .verify img{flex:none}');
}
