import { api } from '$lib/api';
import { rxApi } from '$lib/api/rx';
import QRCode from 'qrcode';
import { printHtml } from '$lib/printer/ticket';
import type { Issuer, Patient } from '$lib/types';
import { consentHtml, privacyNoticeHtml } from './avisos';
import { expedienteHtml } from './expediente';
import { recetaHtml } from './receta';

export { escapeHtml } from './base';
export { consentHtml, expedienteHtml, privacyNoticeHtml, recetaHtml };

export async function printReceta(prescriptionId: string): Promise<void> {
  const r = await rxApi.printData(prescriptionId);
  // The QR is drawn in the browser; the receta still prints if it cannot be generated.
  const qr = r.verify_url ? await QRCode.toDataURL(r.verify_url, { margin: 1, width: 252, errorCorrectionLevel: 'M' }).catch(() => '') : '';
  await printHtml(recetaHtml(r.prescription, r.patient, r.clinic, qr ? { url: r.verify_url, qr } : undefined));
}

export async function printExpediente(patientId: string): Promise<void> {
  const [rec, schema] = await Promise.all([api.patients.record(patientId), api.patients.schema()]);
  await printHtml(expedienteHtml(rec, schema));
}

/** Builds the establishment block when the caller has no Issuer at hand. */
export async function loadIssuer(): Promise<Issuer> {
  const [clinic, legal] = await Promise.all([api.clinic(), api.legal.get()]);
  return { name: clinic.name, address: clinic.address, phone: clinic.phone_number, kind: clinic.kind, legal };
}

export async function printPrivacyNotice(patient: Patient | null, issuer?: Issuer): Promise<void> {
  await printHtml(privacyNoticeHtml(patient, issuer ?? (await loadIssuer())));
}

export async function printConsent(patient: Patient | null, issuer?: Issuer, professional?: { name: string; cedula: string }): Promise<void> {
  await printHtml(consentHtml(patient, issuer ?? (await loadIssuer()), professional));
}
