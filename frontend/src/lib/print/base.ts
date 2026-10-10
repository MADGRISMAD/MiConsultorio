import type { FieldDef, FieldValues, Issuer, Patient } from '$lib/types';

export function escapeHtml(v: unknown): string {
  return String(v ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}
export const e = escapeHtml;
/** escapes and keeps line breaks */
export const multiline = (v: unknown) => escapeHtml(v).replace(/\n/g, '<br>');

export const fmtDate = (iso: string | null | undefined) =>
  iso ? new Date(iso.length === 10 ? `${iso}T12:00:00` : iso).toLocaleDateString('es-MX', { day: 'numeric', month: 'long', year: 'numeric' }) : '';
export const fmtDateTime = (iso: string | null | undefined) =>
  iso
    ? new Date(iso).toLocaleString('es-MX', { day: 'numeric', month: 'long', year: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false }) + ' h'
    : '';

import { fullName } from '$lib/format';
export { fullName };

export function ageText(p: Patient): string {
  if (p.age == null) return '';
  if (p.age < 1 && p.birth_date) {
    const b = new Date(p.birth_date);
    const m = Math.max(0, (new Date().getFullYear() - b.getFullYear()) * 12 + new Date().getMonth() - b.getMonth());
    return `${m} ${m === 1 ? 'mes' : 'meses'}`;
  }
  return `${p.age} ${p.age === 1 ? 'año' : 'años'}`;
}

/** Display value of a field (empty string when nothing was captured). */
export function fieldValue(f: FieldDef, values: FieldValues | undefined): string {
  const v = values?.[f.key];
  if (v == null || v === '' || (Array.isArray(v) && v.length === 0)) return '';
  if (Array.isArray(v)) return v.join(', ');
  if (typeof v === 'boolean') return v ? 'Sí' : 'No';
  if (f.type === 'date' && typeof v === 'string') return fmtDate(v);
  return String(v) + (f.unit ? ` ${f.unit}` : '');
}

export const CSS = `
@page { size: letter; margin: 0; }
.pg { width: 100%; border-collapse: collapse; } /* The browser's own margins stay at 0 (that is what hides its date / title / address header and footer); the sheet's margins are made here, equal on the four sides and repeated on every page. */
.pg > thead > tr > td { height: 10mm; padding: 0; } .pg > tfoot > tr > td { height: 10mm; padding: 0; } .pg > tbody > tr > td { padding: 0 10mm; vertical-align: top; }
* { box-sizing: border-box; }
html { -webkit-print-color-adjust: exact; print-color-adjust: exact; }
body { margin: 0; color: #000; background: #fff; font: 12px/1.45 "Helvetica Neue", Arial, sans-serif; }
h1, h2, h3 { font-family: Georgia, "Times New Roman", serif; font-weight: 700; margin: 0; }
h1 { font-size: 20px; } h2 { font-size: 13px; margin: 16px 0 6px; padding: 4px 8px; border-left: 4px solid #1673D1; background: #EAF2FC; color: #0B2540; border-radius: 0 6px 6px 0; text-transform: uppercase; letter-spacing: .05em; } h3 { font-size: 12.5px; }
p { margin: 0 0 6px; }
.head { position: relative; display: flex; justify-content: space-between; align-items: center; gap: 16px; padding: 12px 16px 18px; margin-bottom: 12px; border-radius: 12px; color: #fff; background: linear-gradient(120deg, #0B2540 0%, #12467F 55%, #1673D1 100%); overflow: hidden; }
.head::after { content: ""; position: absolute; left: 0; right: 0; bottom: 3px; height: 14px; opacity: .35; background: url("data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' width='160' height='14' viewBox='0 0 160 14'><path d='M0 7h52l5-6 7 12 6-9 4 3h86' fill='none' stroke='white' stroke-width='1.4' stroke-linejoin='round'/></svg>") repeat-x; }
.head .est { flex: 1; min-width: 0; } .head .doc { text-align: right; min-width: 170px; }
.head h1, .head .small, .head strong { color: #fff; } .head .small { opacity: .9; }
.brand { display: flex; align-items: center; gap: 12px; }
.brand .logo { flex: none; width: 56px; height: 56px; border-radius: 12px; background: #fff; padding: 4px; object-fit: contain; }
.brand .logo-svg { flex: none; width: 56px; height: 56px; border-radius: 12px; }
.small { font-size: 10.5px; } .muted { color: #333; } .center { text-align: center; }
table { width: 100%; border-collapse: collapse; }
td, th { vertical-align: top; text-align: left; padding: 2px 6px 2px 0; }
.grid { display: grid; grid-template-columns: 1fr 1fr; gap: 3px 18px; }
.grid .full { grid-column: 1 / -1; }
.k { color: #1673D1; font-weight: 700; font-size: 10.5px; display: block; text-transform: uppercase; letter-spacing: .03em; }
.box { border: 1px solid #B9CDE6; background: #F6F9FE; border-radius: 8px; padding: 7px 10px; margin: 8px 0; }
.sig { margin: 38px auto 0; width: 260px; border-top: 1.5px solid #0B2540; padding-top: 3px; font-size: 10.5px; text-align: center; }
.sigs { display: flex; flex-wrap: wrap; gap: 8px 36px; margin-top: 26px; }
.sigs .sig { margin-top: 28px; }
.line { border-bottom: 1px solid #000; height: 22px; margin: 2px 0; }
.nobreak { break-inside: avoid; page-break-inside: avoid; }
.entry { border: 1px solid #B9CDE6; border-left: 4px solid #1673D1; border-radius: 6px; padding: 7px 10px; margin: 0 0 10px; break-inside: avoid; page-break-inside: avoid; }
.entry.add { margin-left: 22px; border-style: dashed; }
.chip { display: inline-block; border: 1px solid #1673D1; color: #0B2540; background: #EAF2FC; border-radius: 3px; padding: 0 5px; margin-right: 4px; font-size: 10.5px; }
.footer { position: fixed; left: 0; right: 0; bottom: 7mm; font-size: 8.5px; text-align: center; color: #333; }
.watermark { position: fixed; top: 38%; left: 0; right: 0; text-align: center; font: 700 110px Arial, sans-serif; color: rgba(0,0,0,.12); transform: rotate(-28deg); letter-spacing: 6px; z-index: 0; pointer-events: none; }
.cb { display: inline-block; width: 13px; height: 13px; border: 1.5px solid #000; vertical-align: -2px; margin-right: 6px; }
`;

export function doc(title: string, body: string, extraCss = ''): string {
  return `<!doctype html><html lang="es-MX"><head><meta charset="utf-8"><title>${e(title)}</title><style>${CSS}${extraCss}</style></head><body><table class="pg"><thead><tr><td></td></tr></thead><tbody><tr><td>${body}</td></tr></tbody><tfoot><tr><td></td></tr></tfoot></table></body></html>`;
}

export const FOOTER_CONF =
  'Documento confidencial. Datos personales y sensibles protegidos por la LFPDPPP. Conservar al menos 5 años desde el último acto médico (NOM-004-SSA3-2012).';

/** Establishment block: name, address, phone, responsable sanitario, aviso de funcionamiento. */
/** The Caresia mark, printed when the clinic has not chosen a logo of its own. */
const CARESIA_MARK = `<svg class="logo-svg" viewBox="0 0 32 32" aria-label="Caresia"><rect width="32" height="32" rx="9" fill="#fff"/><path d="M16 8v16M8 16h16" stroke="#0B2540" stroke-width="3.2" stroke-linecap="round"/><circle cx="23.5" cy="8.5" r="2.5" fill="#1673D1"/></svg>`;

export function issuerBlock(c: Issuer): string {
  const l = c.legal ?? ({} as Issuer['legal']);
  const resp = l.responsible_name
    ? `Responsable sanitario: ${e(l.responsible_name)}${l.responsible_license ? `, céd. prof. ${e(l.responsible_license)}` : ''}${l.responsible_institution ? ` (${e(l.responsible_institution)})` : ''}`
    : '';
  const logo = c.logo ? `<img class="logo" src="${e(c.logo)}" alt="Logo de ${e(c.name)}">` : CARESIA_MARK;
  return `<div class="est"><div class="brand">${logo}<div><h1>${e(c.name)}</h1>
${c.address ? `<div class="small">${e(c.address)}</div>` : ''}
${c.phone ? `<div class="small">Tel. ${e(c.phone)}</div>` : ''}
${resp ? `<div class="small">${resp}</div>` : ''}
${l.operating_notice ? `<div class="small">Aviso de funcionamiento / licencia sanitaria: ${e(l.operating_notice)}</div>` : ''}</div></div></div>`;
}

export function patientBlock(p: Patient): string {
  const animal = p.subject === 'animal';
  const row = (k: string, v: string) => (v ? `<div><span class="k">${e(k)}</span>${e(v)}</div>` : '');
  const species = animal ? String(p.profile?.species ?? p.profile?.especie ?? '') : '';
  const breed = animal ? String(p.profile?.breed ?? p.profile?.raza ?? '') : '';
  if (animal) {
    return `<div class="grid">
${row('Paciente (animal)', fullName(p))}${row('Especie', species)}${row('Raza', breed)}${row('Sexo', p.sex)}${row('Edad', ageText(p))}
${row('Propietario', p.guardian_name)}${row('Teléfono del propietario', p.guardian_phone || p.phone)}</div>`;
  }
  return `<div class="grid">${row('Paciente', fullName(p))}${row('Edad', ageText(p))}${row('Sexo', p.sex)}${row('CURP', p.curp)}
${row('Teléfono', p.phone)}${p.guardian_name ? row('Responsable / tutor', `${p.guardian_name}${p.guardian_relation ? ` (${p.guardian_relation})` : ''}`) : ''}</div>`;
}
