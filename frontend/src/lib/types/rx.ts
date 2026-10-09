import type { Issuer, Patient, Prescription, RxControl, RxItem } from '$lib/types';

export interface Concentration {
  label: string;
  mg_per_ml: number;
}

/** One entry of the medication reference catalog (or one of the clinic's own). */
export interface CatalogMed {
  id: string;
  name: string;
  brand?: string;
  subject: 'person' | 'animal';
  species?: string[];
  category: string;
  control: RxControl;
  route: string;
  presentations: string[];
  typical_dose: string;
  /** single dose, mg per kg */
  mg_per_kg?: number;
  max_mg_per_kg_day?: number;
  concentrations?: Concentration[];
  notes?: string;
  source: 'catalog' | 'clinic';
}

export interface Icd10 {
  code: string;
  name: string;
}

export interface ClinicMedInput {
  name: string;
  brand: string;
  subject: 'person' | 'animal';
  species: string[];
  category: string;
  control: 'No' | 'Antibiótico' | 'Fracción III';
  route: string;
  presentations: string[];
  typical_dose: string;
  mg_per_kg: number | null;
  max_mg_per_kg_day: number | null;
  concentrations: Concentration[];
  notes: string;
}

/** An item of a receta as sent to the server: RxItem plus the data of the weight-based check. */
export interface RxItemInput extends RxItem {
  catalog_id?: string;
  dose_mg?: number;
  doses_per_day?: number;
}

export interface RxInput {
  encounter_id?: string;
  diagnosis: string;
  items: RxItemInput[];
  instructions: string;
  next_visit?: string;
  valid_days?: number;
  weight_kg?: number;
  allergy_override_reason?: string;
  dose_override_reason?: string;
  /** giro issuing it (when the person works in several) */
  area?: string;
  /** complements an earlier receta instead of replacing it */
  complementary?: boolean;
}

export interface AllergyConflict {
  medicine: string;
  allergy: string;
  family?: string;
  message: string;
}

export interface DoseWarning {
  medicine: string;
  daily_mg: number;
  max_daily_mg: number;
  max_mg_per_kg_day: number;
  weight_kg: number;
  message: string;
}

/** The server asks the prescriber to confirm: an allergy match or a dose above the reference maximum. */
export type RxCreateResult =
  | { kind: 'created'; prescription: Prescription }
  | { kind: 'allergy'; message: string; conflicts: AllergyConflict[] }
  | { kind: 'dose'; message: string; warnings: DoseWarning[] };

export interface RxPrintData {
  prescription: Prescription;
  patient: Patient;
  clinic: Issuer;
  verify_url: string;
}

export type VerifyStatus = 'vigente' | 'vencida' | 'anulada';

export interface RxVerification {
  status: VerifyStatus;
  folio: number;
  issued_at: string;
  valid_until: string | null;
  clinic_name: string;
  professional_name: string;
  professional_title: string;
  professional_license: string;
  patient_initials: string;
  retained: boolean;
}

/** What the dose calculator hands back once the prescriber confirms it. */
export interface DoseResult {
  dose: string;
  frequency: string;
  dose_mg: number;
  doses_per_day: number;
  /** the liquid presentation used in the calculation, when it comes from the catalog */
  presentation?: string;
}
