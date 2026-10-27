import { browser } from '$app/environment';
import { error } from '@sveltejs/kit';
import { ApiError } from '$lib/api';
import { blogApi, type BlogCard, type BlogPost } from '$lib/api/blog';
import { faroList, faroPost } from '$lib/api/faro';
import type { EntryGenerator, PageLoad } from './$types';

// Los artículos que ya existen al compilar salen con su HTML completo (personas, Google y asistentes de IA ven lo mismo).
// Los nuevos se cargan en el navegador y, para los rastreadores, vercel.json los sirve desde Faro hasta el siguiente despliegue.
export const ssr = true;
export const prerender = 'auto';
export const entries: EntryGenerator = async () => (await faroList()).map((p) => ({ slug: p.slug }));

export type ArticleData = { post: BlogPost | null; related: BlogCard[]; view: 'ready' | 'missing' | 'error' };

export const load: PageLoad = async ({ params }): Promise<ArticleData> => {
  const slug = params.slug;
  // artículos propios: viven en la API de Caresia (solo desde el navegador; al compilar no hay API a la que llamar)
  if (browser) {
    try {
      const r = await blogApi.post(slug);
      return { post: r.post, related: r.related, view: 'ready' };
    } catch (e) {
      if (!(e instanceof ApiError && e.status === 404)) return { post: null, related: [], view: 'error' };
    }
  }
  const f = await faroPost(slug);
  if (f) return { post: f.post, related: f.related, view: 'ready' };
  // al compilar, un artículo que "no existe" es un fallo de red con Faro: mejor detener el despliegue que publicar una página vacía
  if (!browser) error(502, `No se pudo leer el artículo ${slug} desde Faro`);
  return { post: null, related: [], view: 'missing' };
};
