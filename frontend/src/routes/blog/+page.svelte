<script lang="ts">
  import { onMount } from 'svelte';
  import { theme } from '$lib/theme.svelte';
  import Footer from '$lib/landing/Footer.svelte';
  import Nav from '$lib/landing/Nav.svelte';
  import PostCard from '$lib/blog/PostCard.svelte';
  import { blogApi, type BlogCard } from '$lib/api/blog';

  let { data }: { data: { faro: BlogCard[] } } = $props();

  const title = 'Blog · Caresia';
  const description = 'Guías y consejos para administrar tu clínica dental, consultorio médico, nutrición, veterinaria y más: agenda, expedientes, pacientes y crecimiento.';

  $effect(() => theme.init());

  // Datos estructurados del blog con la lista de artículos (los de Faro vienen en el HTML al compilar)
  const SITE = 'https://caresia.mx';
  const blogLd = $derived(JSON.stringify({
    '@context': 'https://schema.org',
    '@type': 'Blog',
    '@id': `${SITE}/blog#blog`,
    name: 'Blog de Caresia',
    url: `${SITE}/blog`,
    description,
    inLanguage: 'es-MX',
    publisher: { '@id': `${SITE}/#org` },
    blogPost: data.faro.map((p) => ({
      '@type': 'BlogPosting',
      headline: p.title,
      url: `${SITE}/blog/${p.slug}`,
      datePublished: p.published_at ?? p.created_at,
      ...(p.image ? { image: p.image.url } : {})
    }))
  }).replace(/</g, '\\u003c'));

  // Artículos propios (escritos con IA y revisados desde el panel de plataforma) y los que publica Faro, del más nuevo al más viejo.
  const newest = (list: BlogCard[]) => [...list].sort((a, b) => (b.published_at ?? b.created_at).localeCompare(a.published_at ?? a.created_at));
  // Los de Faro ya vienen en el HTML; al montar se suman los propios.
  let own = $state<BlogCard[] | null>(null);
  const posts = $derived(own === null && !data.faro.length ? null : newest([...(own ?? []), ...data.faro]));
  onMount(() => {
    blogApi.list().catch(() => [] as BlogCard[]).then((l) => (own = l));
  });
</script>

<svelte:head>
  <title>{title}</title>
  <meta name="description" content={description} />
  <link rel="canonical" href="https://caresia.mx/blog" />
  <meta property="og:url" content="https://caresia.mx/blog" />
  <meta property="og:image" content="https://caresia.mx/og.png" />
  <meta property="og:type" content="website" />
  <!-- eslint-disable-next-line svelte/no-at-html-tags -->
  {@html `<script type="application/ld+json">${blogLd}<` + `/script>`}
  <meta property="og:title" content={title} />
  <meta property="og:description" content={description} />
</svelte:head>

<div class="landing grain min-h-screen overflow-x-clip bg-paper font-body text-ink antialiased selection:bg-signal selection:text-white" data-theme={theme.mode}>
  <Nav home="/" />

  <main class="mx-auto max-w-6xl px-5 pb-28 pt-32 sm:px-8 sm:pt-40">
    <p class="flex items-center gap-2 font-mono text-[11px] uppercase tracking-[0.16em] text-ink-soft">
      <span class="h-1.5 w-1.5 rounded-full bg-signal"></span>Blog
    </p>
    <h1 class="mt-6 font-display text-[clamp(3rem,10vw,6.5rem)] font-normal leading-[0.92] tracking-[-0.035em]">
      Ideas para tu <span class="italic text-signal">consultorio.</span>
    </h1>
    <p class="mt-6 max-w-2xl text-lg leading-relaxed text-ink-soft [text-wrap:pretty]">{description}</p>

    <div class="mx-auto mt-14 max-w-[44rem] border-t border-ink/10 pt-12">
      {#if posts === null}
        <div class="grid gap-4 md:grid-cols-2" aria-hidden="true">
          {#each [0, 1, 2] as i}<div class="h-64 animate-pulse rounded-[26px] bg-ink/5 {i === 0 ? 'md:col-span-2' : ''}"></div>{/each}
        </div>
      {:else if posts.length}
        <ul class="grid gap-4">
          {#each posts as post, i (post.id)}
            <li><PostCard {post} featured={i === 0} /></li>
          {/each}
        </ul>
      {/if}
    </div>

      </main>

  <Footer home="/" />
</div>
