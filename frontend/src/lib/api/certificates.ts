import { request, seg } from '$lib/api';
import type { Issuer, Patient } from '$lib/types';

export type CertKind = 'medical' | 'veterinary';

export interface Certificate {
  id: string;
  patient_id: string;
  kind: CertKind;
  folio: number;
  issued_at: string;
  valid_until: string | null;
  purpose: string;
  statement: string;
  findings: string;
  extra: { aptitude?: string; restrictions?: string; destination?: string; microchip?: string; vaccines?: string };
  author_name: string;
  author_title: string;
  author_license: string;
  author_institution: string;
  author_specialty_license: string;
  voided_at: string | null;
  voided_by: string;
  void_reason: string;
}

export interface CertificateInput {
  purpose: string;
  statement: string;
  findings?: string;
  aptitude?: string;
  restrictions?: string;
  valid_days?: number;
  destination?: string;
  microchip?: string;
  vaccines?: string;
}

export const certificatesApi = {
  list: (patientId: string) => request<{ certificates: Certificate[]; purposes: Record<CertKind, string[]> }>('GET', `/patients/${seg(patientId)}/certificates`),
  create: (patientId: string, body: CertificateInput) => request<{ certificate: Certificate }>('POST', `/patients/${seg(patientId)}/certificates`, body).then((r) => r.certificate),
  printData: (id: string) => request<{ certificate: Certificate; patient: Patient; clinic: Issuer; verify_url: string }>('GET', `/certificates/${seg(id)}`),
  void: (id: string, reason: string) => request<{ ok: boolean }>('POST', `/certificates/${seg(id)}/void`, { reason })
};
