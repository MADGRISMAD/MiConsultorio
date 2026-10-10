import type { Issuer, Patient } from '$lib/types';
import type { Certificate } from '$lib/api/certificates';
import { doc, e, fmtDate, fmtDateTime, issuerBlock, multiline, patientBlock } from './base';

const APTITUDE: Record<string, string> = { apto: 'Apto(a)', no_apto: 'No apto(a)', restricciones: 'Apto(a) con restricciones' };

function verifyHost(url: string): string {
  try {
    return `${new URL(url).host}/verificar`;
  } catch {
    return url;
  }
}

/** The sheet of a medical or veterinary certificate: same frame as the receta, the professional signs by hand. */
export function certificateHtml(c: Certificate, patient: Patient, clinic: Issuer, verify?: { url: string; qr: string }): string {
  const vet = c.kind === 'veterinary';
  const title = vet ? 'Certificado veterinario' : 'Certificado médico';
  const voided = !!c.voided_at;
  const x = c.extra ?? {};
  const rows = [
    vet && x.microchip ? ['Microchip', x.microchip] : null,
    vet && x.destination ? ['Destino', x.destination] : null,
    !vet && x.aptitude ? ['Aptitud', APTITUDE[x.aptitude] ?? x.aptitude] : null,
    !vet && x.restrictions ? ['Restricciones', x.restrictions] : null
  ].filter(Boolean) as string[][];
  const body = `
${voided ? '<div class="watermark">CANCELADO</div>' : ''}
<div class="head">${issuerBlock(clinic)}<div class="doc"><h1>${title}</h1><div><strong>Folio ${e(String(c.folio).padStart(6, '0'))}</strong></div>
<div class="small">Emitido: ${e(fmtDateTime(c.issued_at))}</div>${c.valid_until ? `<div class="small">Vigente hasta: ${e(fmtDate(c.valid_until))}</div>` : ''}</div></div>

<div class="box nobreak">${patientBlock(patient)}</div>
<p><span class="k">Motivo del certificado</span>${e(c.purpose)}</p>
<h2>${vet ? 'Constancia del médico veterinario' : 'Constancia del médico'}</h2>
<p style="font-size:13px;line-height:1.6">${multiline(c.statement)}</p>
${c.findings ? `<p><span class="k">${vet ? 'Hallazgos de la exploración' : 'Exploración'}</span>${multiline(c.findings)}</p>` : ''}
${vet && x.vaccines ? `<p><span class="k">Vacunación y desparasitación</span>${multiline(x.vaccines)}</p>` : ''}
${rows.map(([k, v]) => `<p><span class="k">${e(k)}</span>${multiline(v)}</p>`).join('')}
<p class="small" style="margin-top:12px">${vet ? 'Este certificado lo emite el profesional bajo su responsabilidad. No sustituye al certificado zoosanitario oficial que exigen algunos traslados (SENASICA).' : 'Este certificado lo emite el profesional bajo su responsabilidad. No es una incapacidad ni sustituye otros trámites oficiales.'}</p>
${voided ? `<div class="box"><strong>Certificado cancelado</strong> el ${e(fmtDateTime(c.voided_at))}${c.voided_by ? ` por ${e(c.voided_by)}` : ''}.${c.void_reason ? ` Motivo: ${e(c.void_reason)}` : ''}</div>` : ''}

<div class="box nobreak" style="margin-top:14px"><span class="k">${vet ? 'Médico veterinario' : 'Profesional'}</span>
<strong>${e(c.author_name)}</strong>${c.author_title ? `<div>${e(c.author_title)}</div>` : ''}
<div class="small">Cédula profesional: <strong>${e(c.author_license)}</strong>${c.author_institution ? ` · Título expedido por ${e(c.author_institution)}` : ''}${c.author_specialty_license ? ` · Cédula de especialidad: ${e(c.author_specialty_license)}` : ''}</div></div>

${verify ? `<div class="verify nobreak"><img src="${e(verify.qr)}" alt="Código QR de verificación" width="84" height="84"><div class="small"><strong>Verifica este certificado en ${e(verifyHost(verify.url))}</strong><br>Escanea el código o abre la dirección: confirma que el folio, el profesional y la vigencia son auténticos. No muestra el contenido del certificado.<br>${e(verify.url)}</div></div>` : ''}
<div class="sig nobreak">Firma autógrafa del ${vet ? 'Médico Veterinario' : 'médico / profesional'}<br>${e(c.author_name)}</div>`;
  return doc(`${title} ${c.folio}`, body, '.verify{display:flex;gap:10px;align-items:center;margin-top:18px} .verify img{flex:none}');
}
