export interface RptCount {
  key: string;
  label: string;
  count: number;
}

export interface RptRate {
  key: string;
  label: string;
  total: number;
  no_show: number;
  rate: number;
}

export interface RptProfessional {
  id: string;
  name: string;
  encounters: number;
  appointments: number;
  no_show: number;
  completed: number;
  avg_attention_min: number | null;
  avg_wait_min: number | null;
}

export interface OperationsReport {
  granularity: 'day' | 'week' | 'month';
  scope: 'clinic' | 'own';
  encounters: { total: number; by_period: RptCount[] };
  professionals: RptProfessional[];
  appointments: { total: number; by_status: RptCount[]; by_source: RptCount[]; cancellation_rate: number };
  no_show: {
    eligible: number;
    count: number;
    rate: number;
    by_professional: RptRate[];
    by_service: RptRate[];
    by_weekday: RptRate[];
    by_hour: RptRate[];
  };
  times: { avg_attention_min: number | null; avg_wait_min: number | null; samples: number };
  occupancy: { booked_min: number; available_min: number; rate: number | null };
  peak_hours: { by_hour: RptCount[]; heatmap: number[][] };
}

export interface RptContact {
  id: string;
  file_number: number;
  name: string;
  subject: 'person' | 'animal';
  last_visit: string | null;
  days_since: number | null;
  contact_name: string;
  contact_phone: string;
  contact_email: string;
  reminders_ok: boolean;
}

export interface RptVaccine extends RptContact {
  vaccine: string;
  next_due: string;
  overdue: boolean;
  days_late: number;
}

export interface RptAgeRow {
  key: string;
  label: string;
  female: number;
  male: number;
  other: number;
  unknown: number;
  total: number;
}

export interface PatientsReport {
  clinic_kind: string;
  visits: { patients: number; repeat: number; by_month: { month: string; new: number; returning: number }[]; new_total: number; returning_total: number };
  signups: { total: number; person: number; animal: number; by_month: { month: string; person: number; animal: number }[] };
  inactive: { days: number; total: number; page: number; limit: number; items: RptContact[] };
  vaccines: { available: boolean; total: number; items: RptVaccine[] };
  reasons: RptCount[];
  people: { total: number; by_age: RptAgeRow[] };
  animals: { total: number; by_species: RptCount[]; by_sex: RptCount[] };
}
