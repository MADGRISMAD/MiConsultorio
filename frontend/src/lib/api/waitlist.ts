import { request, seg } from '$lib/api';
import type {
  AppNotification, MonthLoadDay, MonthSlots, PublicWaitlist, ServiceDuration, WaitlistEntry, WaitlistInput, WaitlistJoinInput
} from '$lib/types/waitlist';

export const notificationsApi = {
  list: (unreadOnly = false, limit = 30) =>
    request<{ notifications: AppNotification[]; unread: number }>('GET', `/notifications?limit=${limit}${unreadOnly ? '&unread=1' : ''}`),
  count: () => request<{ unread: number }>('GET', '/notifications/count').then((r) => r.unread),
  read: (id: string) => request<{ unread: number }>('POST', `/notifications/${seg(id)}/read`).then((r) => r.unread),
  readAll: () => request<{ unread: number }>('POST', '/notifications/read-all').then((r) => r.unread)
};

export const waitlistApi = {
  list: (all = false) => request<{ entries: WaitlistEntry[] }>('GET', `/waitlist${all ? '?status=all' : ''}`).then((r) => r.entries),
  create: (input: WaitlistInput) => request<{ entry: WaitlistEntry }>('POST', '/waitlist', input).then((r) => r.entry),
  update: (id: string, input: WaitlistInput) => request<{ entry: WaitlistEntry }>('PUT', `/waitlist/${seg(id)}`, input).then((r) => r.entry),
  remove: (id: string) => request<void>('DELETE', `/waitlist/${seg(id)}`),
  scan: () => request<{ offers: number }>('POST', '/waitlist/scan').then((r) => r.offers),

  // public (no session)
  join: (slug: string, input: WaitlistJoinInput) => request<unknown>('POST', `/public/booking/${seg(slug)}/waitlist`, input),
  view: (token: string) => request<{ waitlist: PublicWaitlist }>('GET', `/public/waitlist/${seg(token)}`).then((r) => r.waitlist),
  act: (token: string, action: 'accept' | 'decline' | 'leave') =>
    request<{ waitlist: PublicWaitlist }>('POST', `/public/waitlist/${seg(token)}`, { action }).then((r) => r.waitlist)
};

export const bookingMonthApi = {
  /** Days of a month (YYYY-MM) with at least one free slot. Leave `professional` empty for any. */
  month: (slug: string, month: string, professional = '', service = '') =>
    request<MonthSlots>(
      'GET',
      `/public/booking/${seg(slug)}/month?month=${seg(month)}${professional ? `&professional=${seg(professional)}` : ''}${service ? `&service=${seg(service)}` : ''}`
    ).then((r) => r.days.map((d) => d.date))
};

export const agendaLoadApi = {
  monthLoad: (month: string, professional = '') =>
    request<{ month: string; days: MonthLoadDay[] }>('GET', `/agenda/month-load?month=${seg(month)}${professional ? `&professional=${seg(professional)}` : ''}`).then((r) => r.days),
  services: () => request<{ services: ServiceDuration[] }>('GET', '/agenda/services').then((r) => r.services),
  setDuration: (id: string, minutes: number | null) =>
    request<unknown>('PUT', `/agenda/services/${seg(id)}/duration`, { duration_minutes: minutes })
};
