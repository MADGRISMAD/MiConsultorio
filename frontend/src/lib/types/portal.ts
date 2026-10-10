/** Patient portal: what a patient or guardian sees after signing in with a code. No clinical notes. */

export interface PortalInfo {
  name: string;
  welcome: string;
  slug: string;
}

export interface PortalPatient {
  id: string;
  name: string;
  subject: 'person' | 'animal';
  species?: string;
}

export interface PortalMe {
  email: string;
  clinic: { name: string; address: string; phone: string; welcome: string; slug: string; cancel_min_hours: number };
  patients: PortalPatient[];
}

export interface PortalAppointment {
  id: string;
  patient_id: string;
  patient_name: string;
  date: string;
  start_hour: string;
  end_hour: string;
  status: 'scheduled' | 'confirmed' | 'arrived' | 'in_progress' | 'completed' | 'no_show' | 'cancelled';
  professional: string;
  service: string;
  can_cancel: boolean;
}

export interface PortalRxItem {
  medicine: string;
  brand: string;
  presentation: string;
  dose: string;
  route: string;
  frequency: string;
  duration: string;
  quantity: string;
  notes: string;
  control: string;
}

export interface PortalPrescription {
  id: string;
  patient_id: string;
  patient_name: string;
  folio: number;
  mode: 'medication' | 'instructions';
  issued_at: string;
  valid_until: string | null;
  items: PortalRxItem[];
  instructions: string;
  next_visit: string | null;
  author_name: string;
  author_title: string;
  author_license: string;
  author_institution: string;
  voided: boolean;
  /** la reemplazó una receta nueva (vencida, no cancelada) */
  superseded?: boolean;
  voided_at: string | null;
  /** giro that issued it, and its name */
  area?: string;
  area_label?: string;
  complementary?: boolean;
}

export interface PortalVaccination {
  id: string;
  patient_id: string;
  patient_name: string;
  kind: 'vaccine' | 'deworming_internal' | 'deworming_external' | 'other';
  name: string;
  applied_on: string;
  next_due: string | null;
  lot: string;
  dose: string;
  administered_by_name: string;
}

export interface PortalSettings {
  enabled: boolean;
  welcome: string;
  slug: string;
}

export interface PortalNutritionPlan {
  id: string;
  patient_id: string;
  patient_name: string;
  created_at: string;
  by: string;
  data: import('./specialty').NutritionPlanData;
}

export interface PortalHistoryItem {
  id: string;
  patient_id: string;
  patient_name: string;
  at: string;
  type: 'consulta' | 'receta' | 'plan' | 'vacuna' | 'cita';
  title: string;
  by: string;
  area: string;
  area_label: string;
}
