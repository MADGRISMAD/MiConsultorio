/** Waitlist, month availability and in-app notifications. */

export interface AppNotification {
  id: string;
  kind: string;
  title: string;
  body: string;
  link: string;
  read: boolean;
  created_at: string;
}

export interface WaitlistOffer {
  id: string;
  professional: string;
  date: string;
  start: string;
  end: string;
  expires_at: string;
}

export type WaitlistStatus = 'waiting' | 'offered' | 'booked' | 'expired' | 'cancelled';

export interface WaitlistEntry {
  id: string;
  patient_id: string | null;
  name: string;
  phone: string;
  email: string;
  professional_id: string | null;
  professional_name: string;
  service_id: string | null;
  service_name: string;
  /** 0 = Sunday ... 6 = Saturday; empty = any day */
  days: number[];
  from_time: string | null;
  to_time: string | null;
  notes: string;
  consent: boolean;
  status: WaitlistStatus;
  created_via: 'staff' | 'online';
  created_at: string;
  offer: WaitlistOffer | null;
}

export interface WaitlistInput {
  patient_id?: string | null;
  name: string;
  phone: string;
  email: string;
  professional_id: string | null;
  service_id: string | null;
  days: number[];
  from_time: string;
  to_time: string;
  notes: string;
  consent: boolean;
  status?: 'waiting' | 'booked' | 'cancelled';
}

export interface WaitlistJoinInput {
  names: string;
  last_names: string;
  phone: string;
  email: string;
  professional_id?: string;
  service_id?: string;
  days: number[];
  from_time: string;
  to_time: string;
  notes: string;
  accept_privacy: boolean;
  accept_notices: boolean;
  website: string;
}

/** What the public page of a waitlist link shows. */
export interface PublicWaitlist {
  status: WaitlistStatus;
  name: string;
  clinic: { name: string; address: string; phone: string };
  expired: boolean;
  offer: { professional: string; date: string; start: string; end: string; expires_at: string } | null;
}

export interface MonthSlots {
  month: string;
  days: { date: string; slots: number }[];
}

export interface MonthLoadDay {
  date: string;
  appointments: number;
  booked_minutes: number;
  capacity_minutes: number;
  load: number;
  blocked: boolean;
}

export interface ServiceDuration {
  id: string;
  name: string;
  duration_minutes: number | null;
}
