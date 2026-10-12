export interface BookingService {
  id: string;
  name: string;
  category: string;
  price_cents?: number;
  /** anticipo que se pide al reservar este servicio */
  deposit_cents?: number;
  duration_minutes?: number;
  /** presencial, videollamada o ambas */
  modality?: 'presencial' | 'videollamada' | 'ambas';
}

export interface BookingDeposit {
  cents: number;
  state: 'pending' | 'paid' | 'refunded' | 'kept' | 'applied';
  /** donde pagar (solo mientras está pendiente) */
  pay_url?: string;
  expires_at?: string;
}

export interface BookingProfessional {
  id: string;
  name: string;
  /** areas (giros) the professional attends */
  areas?: string[];
  /** da consultas por videollamada */
  video?: boolean;
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
  modality?: 'presencial' | 'videollamada';
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
  modality?: 'presencial' | 'videollamada';
  video_url?: string;
  deposit?: BookingDeposit;
}

export interface PublicAppointment {
  clinic: { name: string; address: string; phone: string };
  professional: string;
  service: string;
  patient: string;
  date: string;
  start: string;
  end: string;
  status: 'pending_payment' | 'scheduled' | 'confirmed' | 'arrived' | 'in_progress' | 'completed' | 'no_show' | 'cancelled';
  /** anticipo pagado o por pagar al reservar */
  deposit?: BookingDeposit | null;
  past: boolean;
  can_confirm: boolean;
  can_cancel: boolean;
  cancel_min_hours: number;
  reminders: boolean;
  rebook_slug: string;
  /** enlace de la videollamada (solo citas por video y activas) */
  video_url?: string;
}
