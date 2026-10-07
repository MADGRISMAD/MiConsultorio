export type ArcoKind = 'acceso' | 'rectificacion' | 'cancelacion' | 'oposicion' | 'revocacion';
export type ArcoStatus = 'recibida' | 'en_revision' | 'requiere_info' | 'atendida' | 'negada' | 'vencida';
export type ArcoDeadlineState = 'ok' | 'soon' | 'overdue' | '';

export interface ArcoRequest {
  id: string;
  folio: string;
  kind: ArcoKind;
  patient_id: string | null;
  patient_name: string;
  requester_name: string;
  requester_email: string;
  requester_phone: string;
  identity_verified: boolean;
  identity_method: string;
  identity_verified_at: string | null;
  description: string;
  status: ArcoStatus;
  resolution: '' | 'procedente' | 'improcedente';
  received_at: string;
  due_ack_at: string;
  due_answer_at: string;
  due_execute_at: string | null;
  answered_at: string | null;
  executed_at: string | null;
  response_text: string;
  denial_reason: string;
  handled_by_name: string;
  created_via: 'staff' | 'public';
  open: boolean;
  pending_execute: boolean;
  deadline: 'answer' | 'execute' | '';
  days_left: number | null;
  deadline_state: ArcoDeadlineState;
}

export interface ArcoSummary {
  open: number;
  overdue: number;
  soon: number;
  pending_execute: number;
}

export interface ArcoEvent {
  id: string;
  kind: string;
  actor: string;
  message: string;
  created_at: string;
}

export interface ArcoSettings {
  slug: string;
  path: string;
  url: string;
  privacy_contact_set: boolean;
  mail_enabled: boolean;
}

export interface ArcoPackage {
  folio: string;
  kind: ArcoKind;
  kind_label: string;
  legal_note: string;
  contact: { name: string; email: string; phone: string; address: string };
  patient?: { id: string; name: string };
  steps: { text: string; done: boolean }[];
  export_path?: string;
  retention_note?: string;
  draft_response: string;
  draft_denial: string;
}

export interface ArcoNewInput {
  kind: ArcoKind;
  requester_name: string;
  requester_email: string;
  requester_phone: string;
  description: string;
  patient_id: string;
  received_on: string;
}

export interface ArcoPublicInfo {
  clinic: { name: string; privacy_contact: string; privacy_email: string; privacy_phone: string };
  kinds: { value: ArcoKind; label: string }[];
}

export interface ArcoPublicInput {
  kind: ArcoKind | '';
  requester_name: string;
  requester_email: string;
  requester_phone: string;
  description: string;
  acknowledged: boolean;
  /** Honeypot: stays empty for people. */
  website: string;
}

export interface ArcoPublicStatus {
  folio: string;
  kind: ArcoKind;
  kind_label: string;
  status: ArcoStatus;
  status_label: string;
  clinic: string;
  received_on: string;
  answered: boolean;
}
