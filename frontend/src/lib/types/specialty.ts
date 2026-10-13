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

export type ChartKind =
  | 'odontogram'
  | 'bodymap'
  | 'nutrition_plan'
  | 'scale'
  | 'therapy_plan'
  | 'certificate'
  | 'prenatal'
  | 'periodontogram'
  | 'ortho_visit'
  | 'milestones'
  | 'food_recall'
  | 'exercises'
  | 'problems';

/** Charts without special geometry: the server checks their shape. */
export type GenericChartData = Record<string, unknown>;

export interface NutritionMeal {
  name: string;
  time: string;
  items: string;
  kcal: number;
}
export interface NutritionDay {
  name: string;
  meals: NutritionMeal[];
}
export interface NutritionTargets {
  bmr: number;
  tdee: number;
  adjust: number;
  kcal: number;
  protein_pct: number;
  carb_pct: number;
  fat_pct: number;
  protein_g: number;
  carb_g: number;
  fat_g: number;
  water_liters: number;
  bmi?: number;
  method: string;
  basis: string;
}

export interface ShoppingSection {
  name: string;
  items: { name: string; quantity: string }[];
}

export interface NutritionPlanData {
  /** the week's groceries, made with the menu and kept in step when a meal is changed with the AI */
  shopping?: ShoppingSection[];
  /** people who eat from the menu (for the list) */
  persons?: number;
  goal: string;
  basis: string;
  kcal: number;
  protein_pct: number;
  carb_pct: number;
  fat_pct: number;
  water_liters: number;
  /** plans saved before the weekly format: one typical day */
  meals: NutritionMeal[];
  /** the 7 days of the week */
  days: NutritionDay[];
  /** foods the patient does not like */
  dislikes: string;
  recommendations: string;
  avoid: string;
  supplements: string;
  follow_up_days: number;
}

// ---- 24-hour food recall (nutrition) ----

export interface RecallMeal {
  name: string;
  time: string;
  /** what was eaten and how much, in the patient's words */
  items: string;
}
export interface FoodRecallData {
  /** the day the patient recalls (YYYY-MM-DD) */
  date: string;
  kind: '24h' | 'dia_tipico';
  meals: RecallMeal[];
  water: string;
  notes: string;
}

// ---- exercise sheet (physiotherapy) ----

export interface ExerciseItem {
  name: string;
  /** series */
  sets: string;
  /** repetitions or time */
  reps: string;
  notes: string;
}
export interface ExercisesData {
  items: ExerciseItem[];
  /** how often: «diario», «3 veces por semana»… */
  frequency: string;
  notes: string;
}

// ---- periodontogram ----

/** Six sites per tooth: vestibular distal, middle, mesial and then lingual/palatal distal, middle, mesial. */
export interface PerioTooth {
  absent?: boolean;
  /** probing depth (mm) */
  pd: (number | null)[];
  /** gingival recession (mm) */
  rec: (number | null)[];
  /** bleeding on probing */
  bop: boolean[];
  /** Miller mobility 0-3 */
  mob: number;
  /** furcation involvement 0-3 */
  furc: number;
  plaque: boolean;
}
export interface PerioData {
  teeth: Record<string, PerioTooth>;
  notes: string;
  plaque_pct: number;
  bleeding_pct: number;
}

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

export interface PatientChart<T = OdontogramData | BodymapData | NutritionPlanData | GenericChartData> {
  id: string;
  patient_id: string;
  kind: ChartKind;
  data: T;
  note: string;
  encounter_id: string | null;
  created_by_name: string;
  created_at: string;
  /** the author's professional data (empty for charts saved before it was recorded) */
  author_title?: string;
  author_cedula?: string;
  author_phone?: string;
  author_email?: string;
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

/** The agreed payments of a plan: a down payment (percent of the total) and equal installments. */
export interface PaymentPlan {
  down_pct: number;
  installments: number;
  interval_days: 7 | 14 | 15 | 30;
  start_date: string;
}

export interface TreatmentPlan {
  payment_plan?: PaymentPlan | null;
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
  payment_plan?: PaymentPlan | null;
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

export interface ExercisesAIInput {
  area: string;
  diagnosis: string;
  goal: string;
  level: string;
  equipment: string;
  restrictions: string;
  count: number;
}
export interface ExercisesAIResult extends ExercisesData {
  disclaimer: string;
}
export interface EncounterDraftResult {
  subjective: string;
  exam: string;
  assessment: string;
  plan: string;
  measures: { pain_eva?: number; rom_deg?: number; rom_note?: string };
  disclaimer: string;
}
export interface RehabAdviceResult {
  suggestion: { stance: 'continuar' | 'ajustar' | 'valorar_alta'; summary: string; reasons: string[]; verify: string[] };
  trend: string;
  disclaimer: string;
  generated_at: string;
  based_on: number;
}

// ---- AI helpers per patient (always a draft or a suggestion for the professional) ----

export interface AiParam {
  label: string;
  options?: string[];
  required: boolean;
  hint?: string;
}
export interface AiTaskInfo {
  key: string;
  label: string;
  desc: string;
  draft: boolean;
  param?: AiParam;
  /** why it cannot run yet */
  disabled?: string;
}
export interface AiLine {
  name: string;
  detail: string;
  qty: number;
  catalog_item_id?: string;
  unit_price_cents?: number;
  tax_rate?: number;
  alerts?: string[];
}
export interface AiResult {
  task: string;
  label: string;
  draft: boolean;
  result: { title: string; summary: string; sections: { heading: string; items: string[] }[]; text: string; lines: AiLine[]; notices?: string[] };
  /** numbers computed by the system on which the answer is based */
  basis: string;
  disclaimer: string;
  generated_at: string;
}
