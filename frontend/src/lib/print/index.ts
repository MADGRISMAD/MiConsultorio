import { api } from '$lib/api';
import { specialtyApi } from '$lib/api/specialty';
import type { BodymapData, NutritionPlanData, OdontogramData } from '$lib/types/specialty';
import { rxApi } from '$lib/api/rx';
import QRCode from 'qrcode';
import { printHtml } from '$lib/printer/ticket';
import type { Issuer, Patient } from '$lib/types';
import type { LabOrder } from '$lib/types/lab';
import { consentHtml, privacyNoticeHtml } from './avisos';
import { expedienteHtml } from './expediente';
import { recetaHtml } from './receta';
import { labReportHtml } from './lab';
import { carnetHtml, chartHtml, nutritionPlanHtml, planHtml, signedConsentHtml } from './specialty';

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
  return { name: clinic.name, address: clinic.address, phone: clinic.phone_number, kind: clinic.kind, legal, logo: clinic.image_url };
}

export async function printPrivacyNotice(patient: Patient | null, issuer?: Issuer): Promise<void> {
  await printHtml(privacyNoticeHtml(patient, issuer ?? (await loadIssuer())));
}

export async function printConsent(patient: Patient | null, issuer?: Issuer, professional?: { name: string; cedula: string }): Promise<void> {
  await printHtml(consentHtml(patient, issuer ?? (await loadIssuer()), professional));
}

/** Prints the lab results of a patient (all orders, or just the given ones). */
export async function printLabReport(patient: Patient, orders: LabOrder[], notice: string): Promise<void> {
  await printHtml(labReportHtml(patient, orders, await loadIssuer(), notice));
}

// ---- specialty record: carnet, charts, plans and signed consents ----

export async function printCarnet(patient: Patient): Promise<void> {
  const [list, weights, issuer] = await Promise.all([specialtyApi.vaccinations(patient.id), specialtyApi.weights(patient.id), loadIssuer()]);
  await printHtml(carnetHtml(patient, list.vaccinations, weights, issuer));
}

export async function printChart(
  patient: Patient,
  kind: 'odontogram' | 'bodymap',
  chart: { data: OdontogramData | BodymapData; note: string; at: string; by: string },
  highlight: Set<number> = new Set()
): Promise<void> {
  await printHtml(chartHtml(patient, kind, chart, await loadIssuer(), highlight));
}

export async function printNutritionPlan(patient: Patient, plan: { data: NutritionPlanData; note: string; at: string; by: string; author?: { title: string; cedula: string; phone: string; email: string } }): Promise<void> {
  await printHtml(nutritionPlanHtml(patient, plan, await loadIssuer()));
}

export async function printPlan(patient: Patient, planId: string): Promise<void> {
  const plan = await specialtyApi.plan(planId);
  const consents = await specialtyApi.consents(patient.id);
  const last = consents.filter((c) => c.plan_id === planId).sort((a, b) => b.signed_at.localeCompare(a.signed_at))[0];
  const full = last ? (await specialtyApi.consent(last.id)).consent : null;
  await printHtml(planHtml(plan, patient, await loadIssuer(), full));
}

export async function printSignedConsent(patient: Patient | null, consentId: string): Promise<void> {
  const r = await specialtyApi.consent(consentId);
  await printHtml(signedConsentHtml(r.consent, patient, r.patient_name, await loadIssuer()));
}
