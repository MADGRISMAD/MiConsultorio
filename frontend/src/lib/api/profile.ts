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
  recent: { rating: number; comment: string; public: boolean; date: string; professional: string }[];
}

export interface PublicClinic {
  name: string;
  address: string;
  phone: string;
  areas: string[];
  professionals: { name: string; title: string; photo_url: string }[];
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
  reviews?: { rating: number; comment: string; date: string }[];
}

export const profileApi = {
  get: () => request<{ profile: ClinicProfile; slug: string; public_url: string; review_url: string }>('GET', '/clinic/profile'),
  save: (p: ClinicProfile) => request<{ profile: ClinicProfile; slug: string; public_url: string; review_url: string }>('PUT', '/clinic/profile', p),
  surveys: () => request<SurveySummary>('GET', '/clinic/surveys'),

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
  professionals: { id: string; name: string; title: string; photo: string | null; hidden: boolean; active: boolean }[];
}

export const mediaApi = {
  overview: () => request<MediaOverview>('GET', '/clinic/media'),
  upload: (slot: 'profile' | 'cover' | 'gallery' | 'pro', image: string, userId = '') =>
    request<{ id: string }>('POST', '/clinic/media', { slot, image, ...(userId ? { user_id: userId } : {}) }),
  remove: (id: string) => request<{ ok: boolean }>('DELETE', `/clinic/media/${seg(id)}`),
  setHidden: (userId: string, hidden: boolean) => request<{ ok: boolean }>('PUT', `/clinic/profile/professionals/${seg(userId)}`, { hidden }),
  /** the editor's own preview of a photo (works before the page is published) */
  url: (id: string) => `/api/clinic/media/${seg(id)}`
};
