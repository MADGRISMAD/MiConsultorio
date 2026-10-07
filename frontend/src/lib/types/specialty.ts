export type VaccinationKind = 'vaccine' | 'deworming_internal' | 'deworming_external' | 'other';

export interface Vaccination {
  id: string;
  patient_id: string;
  kind: VaccinationKind;
  name: string;
  applied_on: string;
  next_due: string | null;
  lot: string;
  dose: string;
  administered_by_name: string;
  notes: string;
  catalog_item_id: string | null;
  voided_at: string | null;
  voided_by_name: string;
  void_reason: string;
  created_by_name: string;
  created_at: string;
}

export interface VaccinationInput {
  kind: VaccinationKind;
  name: string;
  applied_on: string;
  next_due?: string;
  lot?: string;
  dose?: string;
  administered_by_name?: string;
  notes?: string;
  catalog_item_id?: string;
}

export interface VaccineSuggestion {
  kind: VaccinationKind;
  name: string;
  interval_days: number;
  note?: string;
}

export interface VaccinationList {
  vaccinations: Vaccination[];
  suggestions: VaccineSuggestion[];
  subject: 'person' | 'animal';
  species: string;
}

export interface DueVaccination {
  vaccination_id: string;
  patient_id: string;
  patient_name: string;
  subject: string;
  kind: VaccinationKind;
  name: string;
  applied_on: string;
  next_due: string;
  days_left: number;
  overdue: boolean;
  contact_name: string;
  phone: string;
  email: string;
  reminders_ok: boolean;
  species: string | null;
}

export interface WeightPoint {
  date: string;
  weight_kg: number;
  encounter_id: string;
}

// ---- charts ----

export type ChartKind = 'odontogram' | 'bodymap';

export type ToothState = 'caries' | 'restauracion' | 'endodoncia' | 'corona' | 'extraccion_indicada' | 'ausente' | 'sellador' | 'implante' | 'fractura' | 'protesis';
export type Surface = 'V' | 'L' | 'P' | 'M' | 'D' | 'O' | 'I';
export type Dentition = 'adult' | 'child' | 'mixed';

export interface ToothData {
  state?: ToothState;
  surfaces?: Partial<Record<Surface, ToothState>>;
  note?: string;
}
export interface OdontogramData {
  dentition: Dentition;
  teeth: Record<string, ToothData>;
}

export type BodyView = 'front' | 'back';
export type FindingKind = 'dolor' | 'contractura' | 'subluxacion' | 'parestesia' | 'otro';
export interface BodyFinding {
  zone: string;
  view: BodyView;
  kind: FindingKind;
  intensity: number;
  note: string;
}
export interface BodymapData {
  zones: BodyFinding[];
}

export interface PatientChart<T = OdontogramData | BodymapData> {
  id: string;
  patient_id: string;
  kind: ChartKind;
  data: T;
  note: string;
  encounter_id: string | null;
  created_by_name: string;
  created_at: string;
}

export interface ChartList {
  latest: PatientChart | null;
  charts: PatientChart[];
}

// ---- plans ----

export type PlanStatus = 'draft' | 'proposed' | 'accepted' | 'in_progress' | 'completed' | 'cancelled';
export type PlanItemStatus = 'pending' | 'done' | 'cancelled';

export interface PlanItem {
  id: string;
  phase: number;
  position: number;
  description: string;
  tooth: string;
  catalog_item_id: string | null;
  qty: number;
  unit_price_cents: number;
  tax_rate: number;
  status: PlanItemStatus;
  version_added: number;
  total_cents: number;
  done_at: string | null;
  done_by_name: string;
  done_encounter_id: string | null;
  cancel_reason: string;
  sale_id: string | null;
}

export interface PlanEvent {
  version: number;
  action: string;
  detail: string;
  actor_name: string;
  created_at: string;
}

export interface TreatmentPlan {
  id: string;
  patient_id: string;
  patient_name: string;
  title: string;
  status: PlanStatus;
  notes: string;
  professional_id: string | null;
  version: number;
  accepted_version: number;
  accepted_at: string | null;
  accepted_by_name: string;
  cancel_reason: string;
  created_by_name: string;
  created_at: string;
  updated_at: string;
  total_cents: number;
  done_cents: number;
  pending_cents: number;
  items: PlanItem[];
  events?: PlanEvent[];
}

export interface PlanItemInput {
  phase: number;
  description: string;
  tooth: string;
  catalog_item_id?: string;
  qty: number;
  unit_price_cents: number;
  tax_rate: number;
}

export interface PlanInput {
  title: string;
  notes: string;
  items: PlanItemInput[];
}

// ---- consent ----

export type ConsentKind = 'procedimiento' | 'plan_tratamiento' | 'privacidad' | 'telemedicina' | 'animal' | 'psicologia';
export type SignerRole = 'paciente' | 'tutor' | 'propietario';

export interface Consent {
  id: string;
  patient_id: string;
  plan_id: string | null;
  kind: ConsentKind;
  text_snapshot?: string;
  signer_name: string;
  signer_role: SignerRole;
  signature_png?: string;
  witness1: string;
  witness2: string;
  content_sha256: string;
  signed_at: string;
  ip: string;
  user_agent: string;
  registered_by_name: string;
}

export interface ConsentInput {
  kind: ConsentKind;
  plan_id?: string;
  text_snapshot: string;
  signer_name: string;
  signer_role: SignerRole;
  signature_png: string;
  witness1?: string;
  witness2?: string;
}

export interface SignatureInput {
  signer_name: string;
  signer_role: SignerRole;
  signature_png: string;
  witness1?: string;
  witness2?: string;
}
