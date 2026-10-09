import { request, seg } from '$lib/api';
import type { BookingInfo, BookingRequest, BookingResult, BookingSlot, PublicAppointment } from '$lib/types/booking';

/** Public endpoints (no session): online booking and the patient's appointment link. */
export const bookingApi = {
  info: (slug: string) => request<{ booking: BookingInfo }>('GET', `/public/booking/${seg(slug)}`).then((r) => r.booking),
  slots: (slug: string, professional: string, date: string, service = '') =>
    request<{ slots: BookingSlot[] }>(
      'GET',
      `/public/booking/${seg(slug)}/availability?professional=${seg(professional)}&date=${seg(date)}${service ? `&service=${seg(service)}` : ''}`
    ).then((r) => r.slots),
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
