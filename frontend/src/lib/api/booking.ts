import { request, seg } from '$lib/api';
import type { BookingInfo, BookingRequest, BookingResult, BookingSlot, PublicAppointment } from '$lib/types/booking';

/** Public endpoints (no session): online booking and the patient's appointment link. */
export const bookingApi = {
  info: (slug: string) => request<{ booking: BookingInfo }>('GET', `/public/booking/${seg(slug)}`).then((r) => r.booking),
  slots: (slug: string, professional: string, date: string, service = '', holder = '') =>
    request<{ slots: BookingSlot[] }>(
      'GET',
      `/public/booking/${seg(slug)}/availability?professional=${seg(professional)}&date=${seg(date)}${service ? `&service=${seg(service)}` : ''}${holder ? `&holder=${seg(holder)}` : ''}`
    ).then((r) => r.slots),
  /** keeps the picked time for this visitor for a few minutes */
  hold: (slug: string, body: { professional_id: string; service_id?: string; date: string; start: string; holder: string }) =>
    request<{ expires_at: string }>('POST', `/public/booking/${seg(slug)}/hold`, body),
  /** is this phone a registered patient? Returns only the names of the pets registered under it. */
  lookup: (slug: string, phone: string) =>
    request<{ person: boolean; several: boolean; professional_id: string; pets: { id: string; name: string; professional_id: string }[] }>('POST', `/public/booking/${seg(slug)}/lookup`, { phone }),
  book: (slug: string, body: BookingRequest) =>
    request<{ appointment: BookingResult }>('POST', `/public/booking/${seg(slug)}/appointments`, body).then((r) => r.appointment),

  appointment: (token: string) => request<{ appointment: PublicAppointment }>('GET', `/public/appointments/${seg(token)}`).then((r) => r.appointment),
  confirm: (token: string) =>
    request<{ appointment: PublicAppointment }>('POST', `/public/appointments/${seg(token)}/confirm`).then((r) => r.appointment),
  cancel: (token: string, reason: string) =>
    request<{ appointment: PublicAppointment }>('POST', `/public/appointments/${seg(token)}/cancel`, { reason }).then((r) => r.appointment),
  optout: (token: string) =>
    request<{ appointment: PublicAppointment }>('POST', `/public/appointments/${seg(token)}/optout`).then((r) => r.appointment),
  /** the unsubscribe link of the birthday greetings */
  unsubscribeInfo: (token: string) => request<{ clinic: string; subscribed: boolean }>('GET', `/public/unsubscribe/${seg(token)}`),
  unsubscribe: (token: string) => request<{ ok: boolean }>('POST', `/public/unsubscribe/${seg(token)}`)
};
