<script lang="ts">
  // Artículo del blog de Caresia: propio (escrito con IA y revisado desde el panel de plataforma) o de FaroContenido.
  import { page } from '$app/state';
  import { theme } from '$lib/theme.svelte';
  import Footer from '$lib/landing/Footer.svelte';
  import Nav from '$lib/landing/Nav.svelte';
  import { registerHref } from '$lib/landing/data';
  import ArticleBody from '$lib/blog/ArticleBody.svelte';
  import PostCard from '$lib/blog/PostCard.svelte';
  import { blogApi, blogDate, type BlogCard, type BlogPost } from '$lib/api/blog';
  import { ApiError } from '$lib/api';
  import { faroPost } from '$lib/api/faro';

  $effect(() => theme.init());

  const slug = $derived(page.params.slug ?? '');
  let post = $state<BlogPost | null>(null);
  let related = $state<BlogCard[]>([]);
  let view = $state<'loading' | 'ready' | 'missing' | 'error'>('loading');

  // al pasar de un artículo a otro (relacionados) se vuelve a cargar
  $effect(() => {
    const s = slug;
    view = 'loading';
    blogApi
      .post(s)
      // si no es un artículo propio, puede ser uno de FaroContenido
      .catch(async (e) => {
        if (e instanceof ApiError && e.status === 404) {
          const f = await faroPost(s);
          if (f) return f;
        }
        throw e;
      })
      .then((r) => {
        if (s !== slug) return;
        post = r.post;
        related = r.related;
        view = 'ready';
      })
      .catch((e) => {
        if (s === slug) view = e instanceof ApiError && e.status === 404 ? 'missing' : 'error';
      });
  });

  const url = $derived(page.url.origin + page.url.pathname);
  // Datos estructurados para Google: artículo, migas de pan y preguntas frecuentes
  const jsonLd = $derived.by(() => {
    if (!post) return '';
    const graph: Record<string, unknown>[] = [
      {
        '@type': 'BlogPosting',
        headline: post.title,
        description: post.description,
        datePublished: post.published_at,
        dateModified: post.updated_at,
        inLanguage: 'es-MX',
        mainEntityOfPage: url,
        articleSection: post.category,
        keywords: post.keyword,
        ...(post.image ? { image: post.image.url } : {}),
        author: { '@type': 'Organization', name: 'Caresia', url: page.url.origin },
        publisher: { '@type': 'Organization', name: 'Caresia', logo: { '@type': 'ImageObject', url: `${page.url.origin}/apple-touch-icon.png` } }
      },
      {
        '@type': 'BreadcrumbList',
        itemListElement: [
          { '@type': 'ListItem', position: 1, name: 'Inicio', item: page.url.origin + '/' },
          { '@type': 'ListItem', position: 2, name: 'Blog', item: page.url.origin + '/blog' },
          { '@type': 'ListItem', position: 3, name: post.title, item: url }
        ]
      }
    ];
    if (post.faqs.length)
      graph.push({
        '@type': 'FAQPage',
        mainEntity: post.faqs.map((f) => ({ '@type': 'Question', name: f.q, acceptedAnswer: { '@type': 'Answer', text: f.a } }))
      });
    // "<" escapado: el JSON va dentro de una etiqueta <script>
    return `<script type="application/ld+json">${JSON.stringify({ '@context': 'https://schema.org', '@graph': graph }).replace(/</g, '\\u003c')}</` + 'script>';
  });
</script>

<svelte:head>
  {#if post}
    <title>{post.title} · Blog de Caresia</title>
    <meta name="description" content={post.description} />
    <link rel="canonical" href={url} />
    <meta property="og:type" content="article" />
    <meta property="og:title" content={post.title} />
    <meta property="og:description" content={post.description} />
    <meta property="og:url" content={url} />
    <meta property="og:site_name" content="Caresia" />
    <meta property="og:locale" content="es_MX" />
    {#if post.published_at}<meta property="article:published_time" content={post.published_at} />{/if}
    {#if post.image}<meta property="og:image" content={post.image.url} />{/if}
    <meta name="twitter:card" content={post.image ? 'summary_large_image' : 'summary'} />
    {@html jsonLd}
  {:else if view === 'missing'}
    <title>Artículo no encontrado · Caresia</title>
    <meta name="robots" content="noindex" />
  {:else}
    <title>Blog · Caresia</title>
  {/if}
</svelte:head>

<div class="landing grain min-h-screen overflow-x-clip bg-paper font-body text-ink antialiased selection:bg-signal selection:text-white" data-theme={theme.mode}>
  <Nav home="/" />

  <main class="mx-auto max-w-6xl px-5 pb-24 pt-32 sm:px-8 sm:pt-40">
    <a href="/blog" class="group inline-flex items-center gap-2 font-mono text-[11px] uppercase tracking-[0.16em] text-ink-soft transition-colors hover:text-ink">
      <svg viewBox="0 0 24 24" class="h-3.5 w-3.5 transition-transform duration-300 group-hover:-translate-x-0.5" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M19 12H5M11 18l-6-6 6-6" /></svg>
      Todos los artículos
    </a>

    {#if view === 'loading'}
      <div class="mt-10 max-w-3xl space-y-5" aria-busy="true" aria-label="Cargando artículo">
        <div class="h-4 w-40 animate-pulse rounded bg-ink/10"></div>
        <div class="h-16 w-full animate-pulse rounded-2xl bg-ink/10"></div>
        <div class="h-16 w-3/4 animate-pulse rounded-2xl bg-ink/10"></div>
        <div class="mt-10 h-40 w-full animate-pulse rounded-2xl bg-ink/5"></div>
      </div>
    {:else if view === 'missing' || view === 'error'}
      <div class="mt-10 max-w-2xl">
        <h1 class="font-display text-[clamp(2.6rem,7vw,4.5rem)] font-normal leading-[0.95] tracking-[-0.03em]">
          {view === 'missing' ? 'Este artículo no existe.' : 'No pudimos cargar el artículo.'}
        </h1>
        <p class="mt-5 text-lg text-ink-soft">{view === 'missing' ? 'Puede que se haya movido o retirado.' : 'Revisa tu conexión e inténtalo de nuevo.'}</p>
        <a href="/blog" class="mt-8 inline-flex h-12 items-center rounded-full bg-ink px-6 text-[15px] font-medium text-paper transition-colors hover:bg-signal">Ver el blog</a>
      </div>
    {:else if post}
      <article class="mt-10">
        <header class="max-w-4xl">
          <p class="flex flex-wrap items-center gap-x-3 gap-y-1 font-mono text-[11px] uppercase tracking-[0.16em] text-ink-soft">
            <span class="text-signal">{post.category || 'Blog'}</span>
            <span aria-hidden="true" class="h-1 w-1 rounded-full bg-ink/25"></span>
            <time datetime={post.published_at ?? post.created_at}>{blogDate(post.published_at ?? post.created_at)}</time>
            <span aria-hidden="true" class="h-1 w-1 rounded-full bg-ink/25"></span>
            <span>{post.reading_minutes} min de lectura</span>
          </p>
          <h1 class="mt-6 font-display text-[clamp(2.6rem,7vw,5.2rem)] font-normal leading-[0.95] tracking-[-0.03em] [text-wrap:balance]">{post.title}</h1>
          <p class="mt-6 max-w-2xl text-lg leading-relaxed text-ink-soft [text-wrap:pretty] sm:text-xl">{post.excerpt}</p>
        </header>

        {#if post.image}
          <img src={post.image.url} alt={post.image.alt} width="1376" height="768" class="mt-10 aspect-video w-full max-w-4xl rounded-[26px] border border-ink/10 bg-ink/5 object-cover" />
        {/if}

        <div class="mt-12 grid gap-12 border-t border-ink/10 pt-12 lg:grid-cols-[minmax(0,1fr)_17rem] lg:gap-16">
          <div class="max-w-[44rem]">
            <ArticleBody body={post.body} faqs={post.faqs} />
          </div>

          <aside class="lg:sticky lg:top-28 lg:self-start">
            <div class="l-deep rounded-[24px] bg-ink p-6 text-paper">
              <p class="font-display text-[26px] leading-[1.05]">Tu consultorio, <span class="italic text-signal">en orden.</span></p>
              <p class="mt-3 text-[14px] leading-relaxed text-paper/70">Agenda, expedientes, recordatorios y cobros en un solo lugar. Para dentistas, médicos, nutriólogos, veterinarias y más.</p>
              <a href={registerHref} class="mt-5 inline-flex h-11 w-full items-center justify-center rounded-full bg-signal text-[14px] font-medium text-white transition-colors hover:bg-paper hover:text-ink">Solicitar demo</a>
              <a href="/" class="mt-2 inline-flex h-11 w-full items-center justify-center rounded-full text-[14px] font-medium text-paper ring-1 ring-inset ring-paper/25 transition-colors hover:bg-paper/10">Conocer Caresia</a>
            </div>
          </aside>
        </div>
      </article>

      {#if related.length}
        <section aria-labelledby="mas" class="mt-24 border-t border-ink/10 pt-12">
          <h2 id="mas" class="font-display text-[clamp(2rem,5vw,3rem)] font-normal leading-none tracking-[-0.02em]">Sigue leyendo</h2>
          <ul class="mt-8 grid gap-4 md:grid-cols-3">
            {#each related as r (r.id)}<li><PostCard post={r} /></li>{/each}
          </ul>
        </section>
      {/if}
    {/if}
  </main>

  <Footer home="/" />
</div>
