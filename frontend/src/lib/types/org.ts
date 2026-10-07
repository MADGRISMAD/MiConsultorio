export interface OrgBranch {
  id: string;
  name: string;
  kind: string;
  phone_number: string;
  address: string;
  is_matrix: boolean;
  suspended: boolean;
  suspended_at: string | null;
  /** the clinic the current session is working in */
  current: boolean;
  people: number;
  created_at: string;
}

export interface OrgOverview {
  organization: { id: string; name: string } | null;
  is_owner: boolean;
  /** can start the organization or add a branch (subject to the plan limit) */
  can_create: boolean;
  current_clinic_id: string;
  branch_limit: number;
  plan: string;
  plan_name: string;
  branches: OrgBranch[];
  active_branches?: number;
}

export interface OrgBranchInput {
  name: string;
  kind: string;
  phone_number: string;
  address: string;
}

export interface OrgSales {
  /** false when the plan has no cobros: no sales are shown */
  enabled: boolean;
  count: number;
  total_cents: number;
  avg_ticket_cents: number;
  void_count: number;
}

export interface OrgAppointments {
  total: number;
  by_status: Record<string, number>;
  eligible: number;
  no_show: number;
  no_show_rate: number;
  cancellation_rate: number;
}

export interface OrgBranchReport {
  id: string;
  name: string;
  is_matrix: boolean;
  suspended: boolean;
  sales: OrgSales;
  appointments: OrgAppointments;
  consultations: number;
  new_patients: number;
}

export interface OrgDay {
  day: string;
  sales_cents: number;
  appointments: number;
  consultations: number;
  new_patients: number;
}

export interface OrgSummary {
  organization: string;
  from: string;
  to: string;
  branches: OrgBranchReport[];
  total: OrgBranchReport;
  daily: OrgDay[];
}
