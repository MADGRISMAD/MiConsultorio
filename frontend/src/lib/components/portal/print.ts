import { printHtml } from '$lib/printer/ticket';
import { recetaHtml } from '$lib/print/receta';
import type { PortalNutritionPlan, PortalPrescription } from '$lib/types/portal';
import { carnetHtml, nutritionPlanHtml } from '$lib/print/specialty';
import type { Issuer, Patient, Prescription } from '$lib/types';
import type { Vaccination } from '$lib/types/specialty';

type ClinicBlock = { name: string; address: string; phone: string };

const stubPatient = (name: string, animal: boolean) =>
  ({ names: name, last_names: '', subject: animal ? 'animal' : 'person', sex: '', curp: '', phone: '', guardian_name: '', guardian_relation: '', profile: {}, age: null }) as unknown as Patient;
const stubIssuer = (c: ClinicBlock) => ({ name: c.name, address: c.address, phone: c.phone, kind: 'GENERAL_MEDICAL', legal: {} }) as unknown as Issuer;

/** The patient's own copy of a receta, with the same design as the clinic's. The signed original stays with the clinic. */
export function portalRecetaHtml(rx: PortalPrescription, clinic: ClinicBlock, animal: boolean): string {
  const full = {
    ...rx,
    encounter_id: null,
    diagnosis: '',
    author_specialty_license: '',
    voided_at: rx.voided_at,
    voided_by: '',
    void_reason: ''
  } as unknown as Prescription;
  return recetaHtml(full, stubPatient(rx.patient_name, animal), stubIssuer(clinic), undefined,
    `Copia para el paciente descargada del portal de ${clinic.name}. Esta copia no lleva la firma autógrafa del profesional; para surtir medicamentos controlados o retenidos presenta la receta firmada que te entregó el consultorio.`);
}

export async function printPortalReceta(rx: PortalPrescription, clinic: ClinicBlock, animal: boolean): Promise<void> {
  await printHtml(portalRecetaHtml(rx, clinic, animal));
}

type CarnetRow = { name: string; applied_on: string; next_due: string | null; lot: string; dose: string; administered_by_name: string; kind?: string };

export function portalCarnetHtml(patientName: string, clinicName: string, rows: CarnetRow[]): string {
  const list = rows.map((r) => ({ ...r, kind: r.kind ?? 'vaccine', notes: '', voided_at: null })) as unknown as Vaccination[];
  const issuer = { name: clinicName, address: '', phone: '', kind: 'GENERAL_MEDICAL', legal: {} } as unknown as Issuer;
  return carnetHtml(stubPatient(patientName, list.some((v) => v.kind.startsWith('deworming'))), list, [], issuer, `Copia para el paciente descargada del portal de ${clinicName}.`);
}

export async function printPortalCarnet(patientName: string, clinicName: string, rows: CarnetRow[]): Promise<void> {
  await printHtml(portalCarnetHtml(patientName, clinicName, rows));
}

/** The patient's own copy of a nutrition plan, with the same design as the clinic's. */
export async function printPortalPlan(plan: PortalNutritionPlan, clinic: ClinicBlock): Promise<void> {
  const patient = { names: plan.patient_name, last_names: '', subject: 'person', sex: '', curp: '', phone: '', guardian_name: '', guardian_relation: '', profile: {}, age: null } as unknown as Patient;
  const issuer = { name: clinic.name, address: clinic.address, phone: clinic.phone, kind: 'NUTRITION', legal: {} } as unknown as Issuer;
  await printHtml(nutritionPlanHtml(patient, { data: plan.data, note: '', at: plan.created_at, by: plan.by }, issuer));
}
