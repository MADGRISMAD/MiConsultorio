export interface BookingService {
  id: string;
  name: string;
  category: string;
  price_cents?: number;
  duration_minutes?: number;
}

export interface BookingProfessional {
  id: string;
  name: string;
  /** areas (giros) the professional attends */
  areas?: string[];
}

export interface BookingInfo {
  clinic: { name: string; kind: string; address: string; phone: string };
  message: string;
  requires_confirmation: boolean;
  services: BookingService[];
  professionals: BookingProfessional[];
  /** the areas to choose from; empty when the clinic has a single one */
  areas?: { id: string; label: string }[];
  /** the clinic sees pets / people */
  animals?: boolean;
  people?: boolean;
  species?: string[];
  today: string;
  lead_hours: number;
  horizon_days: number;
}

export interface BookingSlot {
  start: string;
  end: string;
}

export interface BookingRequest {
  professional_id: string;
  service_id?: string;
  date: string;
  start: string;
  names: string;
  last_names: string;
  phone: string;
  email: string;
  reason: string;
  accept_privacy: boolean;
  accept_reminders: boolean;
  website: string;
  holder?: string;
  registered?: boolean;
  patient_id?: string;
  animal?: boolean;
  species?: string;
  pet_name?: string;
}

export interface BookingResult {
  token: string;
  date: string;
  start: string;
  end: string;
  professional: string;
  clinic: string;
  status: string;
  pending_confirmation: boolean;
  reminders: boolean;
}

export interface PublicAppointment {
  clinic: { name: string; address: string; phone: string };
  professional: string;
  service: string;
  patient: string;
  date: string;
  start: string;
  end: string;
  status: 'scheduled' | 'confirmed' | 'arrived' | 'in_progress' | 'completed' | 'no_show' | 'cancelled';
  past: boolean;
  can_confirm: boolean;
  can_cancel: boolean;
  cancel_min_hours: number;
  reminders: boolean;
  rebook_slug: string;
}
