export type LabFlag = 'normal' | 'bajo' | 'alto' | 'critico' | 'anormal' | 'na';
export type LabStatus = 'solicitado' | 'parcial' | 'completo' | 'cancelado';

export interface LabBound {
  low: number | null;
  high: number | null;
}

export interface LabAnalyteDef {
  name: string;
  unit: string;
  kind: 'num' | 'text';
  ref?: LabBound;
  ref_male?: LabBound;
  ref_female?: LabBound;
  expected?: string;
  note?: string;
}

export interface LabPanelDef {
  id: string;
  name: string;
  audience: 'person' | 'animal';
  species?: string;
  note?: string;
  analytes: LabAnalyteDef[];
}

export interface LabCatalog {
  version: string;
  notice: string;
  panels: LabPanelDef[];
}

export interface LabResult {
  id: string;
  order_id: string;
  panel: string;
  analyte: string;
  value_num: number | null;
  value_text: string;
  unit: string;
  ref_low: number | null;
  ref_high: number | null;
  ref_source: '' | 'laboratorio' | 'catalogo';
  flag: LabFlag;
  resulted_at: string;
  notes: string;
  supersedes_id: string | null;
  superseded_by: string | null;
  created_by_name: string;
  created_at: string;
}

export interface LabOrder {
  id: string;
  patient_id: string;
  encounter_id: string | null;
  title: string;
  status: LabStatus;
  ordered_at: string;
  ordered_by_name: string;
  lab_name: string;
  notes: string;
  attachment_id: string | null;
  cancel_reason: string;
  cancelled_at: string | null;
  cancelled_by: string;
  created_at: string;
  /** estudios solicitados en la orden (lo que lista la hoja impresa) */
  requested: string[];
  results: LabResult[];
}

export interface LabResultInput {
  panel: string;
  analyte: string;
  value_num?: number | null;
  value_text?: string;
  unit?: string;
  ref_low?: number | null;
  ref_high?: number | null;
  ref_source?: '' | 'laboratorio' | 'catalogo';
  flag?: LabFlag;
  resulted_at?: string;
  notes?: string;
  supersedes_id?: string;
}

export interface LabOrderInput {
  title: string;
  lab_name?: string;
  notes?: string;
  attachment_id?: string;
  ordered_at?: string;
  requested?: string[];
  results?: LabResultInput[];
  complete?: boolean;
}

export interface LabTrendPoint {
  id: string;
  order_id: string;
  resulted_at: string;
  value: number;
  unit: string;
  ref_low: number | null;
  ref_high: number | null;
  ref_source: string;
  flag: LabFlag;
  lab_name: string;
}

export interface LabAnalyteSummary {
  analyte: string;
  unit: string;
  count: number;
  last_at: string;
}

export interface LabTrends {
  analyte: string;
  points: LabTrendPoint[];
  analytes: LabAnalyteSummary[];
}

// ---- growth ----

export type GrowthIndicatorKey = 'weight_for_age' | 'length_height_for_age' | 'bmi_for_age' | 'head_circumference_for_age';

export interface GrowthMeasurement {
  encounter_id: string;
  date: string;
  age_months: number | null;
  weight_kg: number | null;
  height_cm: number | null;
  bmi: number | null;
  head_cm: number | null;
}

export interface GrowthPoint {
  date: string;
  age_months: number | null;
  value: number;
  z: number | null;
  percentile: number | null;
}

export interface GrowthCurvePoint {
  age_months: number;
  value: number;
}

export interface GrowthIndicator {
  indicator: GrowthIndicatorKey;
  unit: string;
  has_reference: boolean;
  points: GrowthPoint[];
  curves: Record<string, GrowthCurvePoint[]>;
  ref_min_age_months: number | null;
  ref_max_age_months: number | null;
}

export interface GrowthStandard {
  standard: string;
  version: number;
  source_name: string;
  imported_at: string;
}

export type GrowthReferenceStatus = 'ok' | 'animal' | 'no_sex' | 'no_birth_date' | 'no_references';

export interface PatientGrowth {
  subject: 'person' | 'animal';
  sex: string;
  birth_date: string | null;
  age_months?: number | null;
  measurements: GrowthMeasurement[];
  standards: GrowthStandard[];
  standard: string;
  indicators: Partial<Record<GrowthIndicatorKey, GrowthIndicator>>;
  reference_status: GrowthReferenceStatus;
}

export interface GrowthImportRecord {
  id: string;
  standard: string;
  version: number;
  source_name: string;
  file_name: string;
  sha256: string;
  row_count: number;
  created_by_name: string;
  created_at: string;
  /** viene incluida con Caresia (no la cargó el consultorio) */
  platform?: boolean;
}

export interface GrowthCoverage {
  indicator: string;
  sex: string;
  rows: number;
  min_age_months: number;
  max_age_months: number;
  has_lms: boolean;
}

export interface GrowthImportPreview {
  valid: boolean;
  errors: { line: number; message: string }[];
  error_count: number;
  rows: number;
  has_lms: boolean;
  groups: { indicator: string; sex: string; rows: number; min_age_months: number; max_age_months: number }[];
  sample: Record<string, number | string>[];
  standard: string;
  next_version: number;
  sha256: string;
  columns: string[];
}

export interface GrowthImportInput {
  standard: string;
  source_name: string;
  file_name: string;
  csv: string;
  confirm: boolean;
}

/** Lo que la IA leyó de un reporte de laboratorio (nada se guarda hasta que el profesional lo revisa). */
export interface LabScanRow {
  section: string;
  analyte: string;
  value_num: number | null;
  value_text: string;
  unit: string;
  ref_low: number | null;
  ref_high: number | null;
  flag: string;
}

export interface LabScan {
  study: string;
  lab_name: string;
  /** AAAA-MM-DD, vacío si el documento no la trae */
  date: string;
  results: LabScanRow[];
}
