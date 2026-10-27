<script lang="ts">
  // Página pública de una especialidad: mismo diseño que las páginas legales y el blog (menú, pie, tipografías y colores de la portada).
  import { theme } from '$lib/theme.svelte';
  import Footer from './Footer.svelte';
  import Nav from './Nav.svelte';
  import Icon from './Icon.svelte';
  import { faqs, plans, registerHref } from './data';
  import { especialidades, type Especialidad } from './especialidades';

  let { e }: { e: Especialidad } = $props();

  const SITE = 'https://caresia.mx';
  const url = $derived(SITE + e.path);
  // Lo que incluye todo plan, tomado de la lista del plan Básico (no se repite aquí lo que es solo de otros planes)
  const incluye = plans[0].features.slice(2, 9);
  const preguntas = faqs.slice(0, 6);
  const otras = $derived(especialidades.filter((x) => x.path !== e.path));

  const ld = $derived(
    JSON.stringify({
      '@context': 'https://schema.org',
      '@graph': [
        {
          '@type': 'SoftwareApplication',
          '@id': `${url}#app`,
          name: `Caresia para ${e.tab.toLowerCase()}`,
          applicationCategory: 'HealthApplication',
          operatingSystem: 'Web',
          inLanguage: 'es-MX',
          url,
          description: e.description,
          publisher: { '@id': `${SITE}/#org` },
          offers: plans.map((p) => {
            const price = p.price.replace(/[^0-9.]/g, '');
            return { '@type': 'Offer', name: p.name, price, priceCurrency: 'MXN', availability: 'https://schema.org/InStock', url: `${SITE}/#precios`, priceSpecification: { '@type': 'UnitPriceSpecification', price, priceCurrency: 'MXN', unitText: 'MONTH', valueAddedTaxIncluded: false } };
          })
        },
        {
          '@type': 'BreadcrumbList',
          itemListElement: [
            { '@type': 'ListItem', position: 1, name: 'Inicio', item: `${SITE}/` },
            { '@type': 'ListItem', position: 2, name: 'Especialidades', item: `${SITE}/#especialidades` },
            { '@type': 'ListItem', position: 3, name: e.tab, item: url }
          ]
        }
      ]
    }).replace(/</g, '\\u003c')
  );

  $effect(() => theme.init());
</script>

<svelte:head>
  <title>{e.title}</title>
  <meta name="description" content={e.description} />
  <link rel="canonical" href={url} />
  <meta property="og:type" content="website" />
  <meta property="og:site_name" content="Caresia" />
  <meta property="og:locale" content="es_MX" />
  <meta property="og:url" content={url} />
  <meta property="og:title" content={e.title} />
  <meta property="og:description" content={e.description} />
  <meta property="og:image" content="{SITE}/og.png" />
  <meta name="twitter:card" content="summary_large_image" />
  <!-- eslint-disable-next-line svelte/no-at-html-tags -->
  {@html `<script type="application/ld+json">${ld}<` + `/script>`}
</svelte:head>

<div class="landing grain min-h-screen overflow-x-clip bg-paper font-body text-ink antialiased selection:bg-signal selection:text-white" data-theme={theme.mode}>
  <Nav home="/" />

  <main class="mx-auto max-w-6xl px-5 pb-28 pt-32 sm:px-8 sm:pt-40">
    <p class="flex items-center gap-2 font-mono text-[11px] uppercase tracking-[0.16em] text-ink-soft"><span class="h-1.5 w-1.5 rounded-full bg-signal"></span>Caresia para {e.tab.toLowerCase()}</p>
    <h1 class="mt-6 font-display text-[clamp(2.75rem,8vw,6rem)] font-normal leading-[0.95] tracking-[-0.03em] [text-wrap:balance]">{e.h1}</h1>
    <p class="mt-6 max-w-2xl text-lg leading-relaxed text-ink-soft [text-wrap:pretty] sm:text-xl">{e.lead}</p>
    <div class="mt-8 flex flex-wrap items-center gap-3">
      <a href={registerHref} class="inline-flex h-12 items-center rounded-full bg-ink px-7 text-[15px] font-medium text-paper transition-colors hover:bg-signal">Solicitar demo gratis</a>
      <a href="/#precios" class="inline-flex h-12 items-center rounded-full px-6 text-[15px] font-medium text-ink ring-1 ring-inset ring-ink/20 transition-colors hover:bg-ink/5">Ver precios</a>
    </div>

    <section aria-labelledby="hecho" class="mt-20 border-t border-ink/10 pt-12">
      <h2 id="hecho" class="font-display text-[clamp(2rem,5vw,3.25rem)] font-normal leading-none tracking-[-0.02em]">Hecho para tu práctica</h2>
      <p class="mt-5 max-w-2xl text-lg leading-relaxed text-ink-soft [text-wrap:pretty]">{e.text} Elige tu giro y Caresia ajusta formularios, secciones y reportes; si atiendes más de uno, conviven en el mismo consultorio.</p>
      <ul class="mt-8 grid gap-4 md:grid-cols-2">
        {#each e.chips as c}
          <li class="flex gap-4 rounded-[24px] bg-panel p-6 ring-1 ring-ink/10"><Icon name="check" class="mt-1 h-5 w-5 flex-none text-signal" strokeWidth={2.4} /><span class="text-[17px] font-medium leading-snug">{c}</span></li>
        {/each}
      </ul>
    </section>

    <section aria-labelledby="incluye" class="mt-20 border-t border-ink/10 pt-12">
      <h2 id="incluye" class="font-display text-[clamp(2rem,5vw,3.25rem)] font-normal leading-none tracking-[-0.02em]">Lo que ya incluye desde el plan Básico</h2>
      <ul class="mt-8 grid gap-x-10 gap-y-3 md:grid-cols-2">
        {#each incluye as f}<li class="flex gap-3 text-[16px] leading-snug text-ink-soft"><Icon name="check" class="mt-0.5 h-4 w-4 flex-none text-mint" strokeWidth={2.4} />{f}</li>{/each}
      </ul>
    </section>

    <section aria-labelledby="precios" class="mt-20 border-t border-ink/10 pt-12">
      <h2 id="precios" class="font-display text-[clamp(2rem,5vw,3.25rem)] font-normal leading-none tracking-[-0.02em]">Precios en pesos, sin sorpresas</h2>
      <p class="mt-5 max-w-2xl text-lg leading-relaxed text-ink-soft">Tres planes mensuales, más IVA. Empiezas en el Básico y cambias de plan cuando lo necesites.</p>
      <ul class="mt-8 grid gap-4 md:grid-cols-3">
        {#each plans as p}
          <li class="rounded-[24px] bg-panel p-6 ring-1 ring-ink/10">
            <p class="font-mono text-[11px] uppercase tracking-[0.16em] text-ink-soft">{p.name}</p>
            <p class="mt-3 font-display text-5xl leading-none">{p.price}<span class="ml-2 font-body text-[14px] text-ink-soft">{p.period}</span></p>
            <p class="mt-3 text-[15px] text-ink-soft">{p.tagline}</p>
          </li>
        {/each}
      </ul>
      <p class="mt-6 text-[15px] text-ink-soft">Detalle completo de cada plan en <a href="/#precios" class="text-ink underline decoration-ink/30 underline-offset-4 hover:decoration-ink">la sección de precios</a>.</p>
    </section>

    <section aria-labelledby="preguntas" class="mt-20 border-t border-ink/10 pt-12">
      <h2 id="preguntas" class="font-display text-[clamp(2rem,5vw,3.25rem)] font-normal leading-none tracking-[-0.02em]">Preguntas frecuentes</h2>
      <dl class="mt-8 max-w-3xl divide-y divide-ink/10">
        {#each preguntas as f}
          <div class="py-6"><dt class="text-[18px] font-medium text-ink">{f.q}</dt><dd class="mt-2 text-[16px] leading-relaxed text-ink-soft [text-wrap:pretty]">{f.a}</dd></div>
        {/each}
      </dl>
    </section>

    <section aria-labelledby="leer" class="mt-20 border-t border-ink/10 pt-12">
      <h2 id="leer" class="font-display text-[clamp(2rem,5vw,3.25rem)] font-normal leading-none tracking-[-0.02em]">Para seguir leyendo</h2>
      <ul class="mt-8 grid gap-3 text-[17px]">
        {#each e.articles as a}<li><a href="/blog/{a.slug}" class="text-ink underline decoration-ink/30 underline-offset-4 hover:decoration-ink">{a.title}</a></li>{/each}
        <li><a href="/blog" class="text-ink-soft underline decoration-ink/20 underline-offset-4 hover:text-ink">Todos los artículos del blog</a></li>
      </ul>
      <p class="mt-10 font-mono text-[11px] uppercase tracking-[0.16em] text-ink-soft">Otras especialidades</p>
      <ul class="mt-3 flex flex-wrap gap-2">
        {#each otras as o}<li><a href={o.path} class="inline-flex h-10 items-center rounded-full px-4 text-[14px] text-ink ring-1 ring-inset ring-ink/20 transition-colors hover:bg-ink/5">{o.tab}</a></li>{/each}
      </ul>
    </section>
  </main>

  <Footer home="/" />
</div>
