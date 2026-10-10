import { request, seg } from '$lib/api';

export interface ClinicProfile {
  enabled: boolean;
  tagline: string;
  about: string;
  hours_text: string;
  whatsapp: string;
  contact_email: string;
  website: string;
  maps_url: string;
  google_place_id: string;
  show_reviews: boolean;
  survey_enabled: boolean;
  survey_delay_hours: number;
  maps_min_rating: number;
  // directorio
  listed: boolean;
  city: string;
  state: string;
  neighborhood: string;
  insurances: string[];
  languages: string[];
  payment_methods: string[];
  /** ids de los servicios que se muestran con precio (solo al guardar) */
  public_services?: string[];
}

export interface ProfileService {
  id: string;
  name: string;
  price_cents: number;
  public: boolean;
}

export interface SurveyStats {
  count: number;
  average: number;
  distribution: number[];
}

export interface SurveySummary {
  stats: SurveyStats;
  sent: number;
  answered: number;
  recent: { id: string; rating: number; comment: string; public: boolean; date: string; professional: string; reply: string }[];
}

export interface PublicClinic {
  name: string;
  address: string;
  phone: string;
  areas: string[];
  professionals: { name: string; title: string; photo_url: string; cedula: string; cedula_specialty: string; bio: string }[];
  tagline: string;
  about: string;
  hours_text: string;
  whatsapp: string;
  email: string;
  website: string;
  maps_url: string;
  review_url: string;
  kinds: string[];
  profile_url: string;
  cover_url: string;
  gallery: string[];
  booking_url: string;
  slug: string;
  rating?: SurveyStats;
  reviews?: { rating: number; comment: string; date: string; reply: string; verified: boolean }[];
  city: string;
  state: string;
  neighborhood: string;
  listed: boolean;
  insurances: string[];
  languages: string[];
  payment_methods: string[];
  services: { name: string; price_cents: number; duration_minutes: number | null }[];
}

export interface DirectoryHit {
  slug: string;
  name: string;
  tagline: string;
  city: string;
  state: string;
  areas: string[];
  photo_url: string;
  cover_url: string;
  rating: SurveyStats;
  booking: boolean;
  next_slot: { date: string; start: string; professional: string } | null;
  price_from_cents: number;
  insurances: string[];
}

export interface DirectoryOptions {
  areas: { code: string; label: string; slug: string }[];
  states: string[];
  cities: { state: string; city: string; slug: string; count: number }[];
  payment_methods: string[];
}

export const directoryApi = {
  search: (f: { q?: string; area?: string; state?: string; city?: string; page?: number }) => {
    const p = new URLSearchParams();
    for (const [k, v] of Object.entries(f)) if (v) p.set(k, String(v));
    return request<{ results: DirectoryHit[]; total: number; page: number; page_size: number }>('GET', `/public/directory?${p}`);
  },
  options: () => request<DirectoryOptions>('GET', '/public/directory/options')
};

export const profileApi = {
  get: () =>
    request<{ profile: ClinicProfile; slug: string; public_url: string; review_url: string; services: ProfileService[]; states: string[]; payment_methods: string[] }>(
      'GET',
      '/clinic/profile'
    ),
  save: (p: ClinicProfile) =>
    request<{ profile: ClinicProfile; slug: string; public_url: string; review_url: string; services: ProfileService[]; states: string[]; payment_methods: string[] }>(
      'PUT',
      '/clinic/profile',
      p
    ),
  surveys: () => request<SurveySummary>('GET', '/clinic/surveys'),
  reply: (id: string, reply: string) => request<{ ok: boolean }>('PUT', `/clinic/surveys/${seg(id)}/reply`, { reply }),

  publicClinic: (slug: string) => request<PublicClinic>('GET', `/public/clinic/${seg(slug)}`),
  survey: (token: string) =>
    request<{ clinic_name: string; professional: string; answered: boolean; rating: number; review_url: string }>('GET', `/public/survey/${seg(token)}`),
  answer: (token: string, body: { rating: number; comment: string; public_ok: boolean }) =>
    request<{ ok: boolean; review_url: string }>('POST', `/public/survey/${seg(token)}`, body)
};

export interface CalendarFeed {
  enabled: boolean;
  show_names: boolean;
  path: string;
}

export const calendarApi = {
  get: () => request<CalendarFeed>('GET', '/me/calendar'),
  set: (action: 'enable' | 'rotate' | 'disable', show_names: boolean) => request<CalendarFeed>('POST', '/me/calendar', { action, show_names })
};

export interface MediaOverview {
  profile: string | null;
  cover: string | null;
  gallery: string[];
  max_gallery: number;
  professionals: { id: string; name: string; title: string; photo: string | null; hidden: boolean; active: boolean; bio: string }[];
}

export const mediaApi = {
  overview: () => request<MediaOverview>('GET', '/clinic/media'),
  upload: (slot: 'profile' | 'cover' | 'gallery' | 'pro', image: string, userId = '') =>
    request<{ id: string }>('POST', '/clinic/media', { slot, image, ...(userId ? { user_id: userId } : {}) }),
  remove: (id: string) => request<{ ok: boolean }>('DELETE', `/clinic/media/${seg(id)}`),
  setHidden: (userId: string, hidden: boolean, bio?: string) =>
    request<{ ok: boolean }>('PUT', `/clinic/profile/professionals/${seg(userId)}`, { hidden, ...(bio !== undefined ? { bio } : {}) }),
  /** the editor's own preview of a photo (works before the page is published) */
  url: (id: string) => `/api/clinic/media/${seg(id)}`
};
