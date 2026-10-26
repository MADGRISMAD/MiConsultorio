import type { BlogBlock, BlogCard, BlogPost } from './blog';

// Artículos que escribe y publica FaroContenido (faro.hyperionmkt.com). Se leen por su API pública y se pintan con
// los componentes del blog de Caresia: Faro solo entrega los datos. Contrato: docs/API.md del repo FaroContenido.
const FARO = 'https://faro.hyperionmkt.com/api/v1/caresia.mx';

interface FaroPost {
  id: string;
  slug: string;
  title: string;
  summary: string;
  published_at: string;
  reading_minutes: number;
  image: { url: string; alt: string } | null;
}
interface FaroFull extends FaroPost {
  seo: { description: string };
  content: { intro: string; sections: { heading: string; level?: number; paragraphs: string[]; bullets: string[] }[]; faq: { question: string; answer: string }[] };
}

const card = (p: FaroPost): BlogCard => ({
  id: `faro-${p.id}`,
  slug: p.slug,
  title: p.title,
  excerpt: p.summary,
  category: '',
  reading_minutes: p.reading_minutes,
  status: 'published',
  published_at: p.published_at,
  created_at: p.published_at,
  image: p.image ? { url: p.image.url, alt: p.image.alt } : null
});

/** Lista de artículos de Faro. Si Faro no responde, devuelve [] y el blog sigue mostrando el resto. */
export async function faroList(): Promise<BlogCard[]> {
  try {
    const r = await fetch(`${FARO}/posts?limit=50`);
    return r.ok ? ((await r.json()).posts as FaroPost[]).map(card) : [];
  } catch {
    return [];
  }
}

/** Un artículo de Faro con la forma del blog propio, o null si no existe. */
export async function faroPost(slug: string): Promise<{ post: BlogPost; related: BlogCard[] } | null> {
  try {
    const r = await fetch(`${FARO}/posts/${encodeURIComponent(slug)}`);
    if (!r.ok) return null;
    const p = (await r.json()) as FaroFull;
    const body: BlogBlock[] = [{ type: 'p', text: p.content.intro }];
    for (const s of p.content.sections) {
      body.push({ type: s.level === 3 ? 'h3' : 'h2', text: s.heading });
      for (const t of s.paragraphs) body.push({ type: 'p', text: t });
      if (s.bullets.length) body.push({ type: 'ul', items: s.bullets });
    }
    const post: BlogPost = {
      ...card(p),
      description: p.seo.description,
      keyword: '',
      topic: '',
      body,
      faqs: p.content.faq.map((f) => ({ q: f.question, a: f.answer })),
      updated_at: p.published_at
    };
    return { post, related: (await faroList()).filter((c) => c.slug !== slug).slice(0, 3) };
  } catch {
    return null;
  }
}
