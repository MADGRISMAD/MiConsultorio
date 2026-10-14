import { request, seg } from '$lib/api';

export interface BlogBlock {
  type: 'h2' | 'h3' | 'p' | 'ul' | 'ol' | 'tip';
  text?: string;
  items?: string[];
}

export interface BlogCard {
  id: string;
  slug: string;
  title: string;
  excerpt: string;
  category: string;
  reading_minutes: number;
  status: 'draft' | 'published';
  published_at: string | null;
  created_at: string;
}

export interface BlogPost extends BlogCard {
  description: string;
  keyword: string;
  topic: string;
  body: BlogBlock[];
  faqs: { q: string; a: string }[];
  updated_at: string;
}

export interface BlogPanel {
  posts: BlogCard[];
  ai_ready: boolean;
  autopilot: boolean;
  auto_publish: boolean;
  next_run: string | null;
  next_topic: string;
  topics_left: number;
  generating: boolean;
  last_error: string;
}

export const blogApi = {
  list: () => request<{ posts: BlogCard[] }>('GET', '/public/blog').then((r) => r.posts),
  post: (slug: string) => request<{ post: BlogPost; related: BlogCard[] }>('GET', `/public/blog/${seg(slug)}`),

  // panel de plataforma
  panel: () => request<BlogPanel>('GET', '/platform/blog'),
  draft: (id: string) => request<{ post: BlogPost }>('GET', `/platform/blog/${seg(id)}`).then((r) => r.post),
  generate: (topic: string, publish: boolean) => request<{ generating: boolean }>('POST', '/platform/blog/generate', { topic, publish }),
  setStatus: (id: string, status: 'draft' | 'published') => request<void>('PATCH', `/platform/blog/${seg(id)}`, { status }),
  remove: (id: string) => request<void>('DELETE', `/platform/blog/${seg(id)}`)
};

/** Fecha larga en español: "13 de octubre de 2026". */
export const blogDate = (iso: string | null) => (iso ? new Date(iso).toLocaleDateString('es-MX', { day: 'numeric', month: 'long', year: 'numeric' }) : '');
