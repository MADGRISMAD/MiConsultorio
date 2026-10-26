<script lang="ts">
  import { onMount } from 'svelte';
  import Lenis from 'lenis';
  import Hero from './Hero.svelte';
  import LazySection from './LazySection.svelte';
  import Nav from './Nav.svelte';
  import { theme } from '$lib/theme.svelte';
  import { i18n } from '$lib/i18n/index.svelte';
  import { translateToEnglish } from './translate';
  import { reducedMotion } from './motion';
  import { plans } from './data';

  const SITE = 'https://caresia.mx';
  const title = 'Caresia · Software para clínicas dentales y consultorios médicos';
  const description = 'Agenda, expedientes, recetas, cobros, reportes e inteligencia artificial en un solo lugar. Para clínicas dentales, consultorios médicos, nutriólogos, veterinarias y clínicas con varias especialidades.';
  // Precios y datos de la empresa en el HTML (sin JavaScript): buscadores y asistentes de IA los leen directo.
  const jsonld = JSON.stringify({
    '@context': 'https://schema.org',
    '@graph': [
      { '@type': 'Organization', '@id': `${SITE}/#org`, name: 'Caresia', url: `${SITE}/`, logo: `${SITE}/icons/icon-512.png`, email: 'caresia-info@caresia.mx', areaServed: 'MX' },
      { '@type': 'WebSite', '@id': `${SITE}/#web`, name: 'Caresia', url: `${SITE}/`, inLanguage: 'es-MX', publisher: { '@id': `${SITE}/#org` } },
      {
        '@type': 'SoftwareApplication', '@id': `${SITE}/#app`, name: 'Caresia', applicationCategory: 'HealthApplication', operatingSystem: 'Web', inLanguage: 'es-MX', url: `${SITE}/`, description,
        publisher: { '@id': `${SITE}/#org` },
        offers: plans.map((p) => ({ '@type': 'Offer', name: p.name, price: p.price.replace(/[^0-9.]/g, ''), priceCurrency: 'MXN', url: `${SITE}/#precios`, description: p.tagline }))
      }
    ]
  }).replace(/</g, '\\u003c');

  let ready = $state(false);
  // La landing está en español; en inglés solo si la persona lo eligió con el botón ES/EN (se recuerda).
  onMount(() => {
    try {
      if (localStorage.getItem('caresia_ui_lang') === 'en') i18n.set('en');
    } catch {
      /* sin almacenamiento: queda en español */
    }
    ready = true;
    return () => i18n.restoreDocumentLang();
  });
  $effect(() => {
    if (!ready || i18n.locale !== 'en') return;
    return translateToEnglish(document.body); // el body entero: incluye el aviso de cookies, que vive fuera de la landing
  });

  /** Inertial smooth scrolling. Off when the user prefers reduced motion. */
  onMount(() => {
    if (reducedMotion()) return;
    const lenis = new Lenis({ duration: 1.15, anchors: { offset: -72 } });
    let id = requestAnimationFrame(function raf(t) {
      lenis.raf(t);
      id = requestAnimationFrame(raf);
    });
    return () => {
      cancelAnimationFrame(id);
      lenis.destroy();
    };
  });
</script>

<svelte:head>
  <title>{title}</title>
  <meta name="description" content={description} />
  <link rel="canonical" href="{SITE}/" />
  <meta property="og:type" content="website" />
  <meta property="og:site_name" content="Caresia" />
  <meta property="og:locale" content="es_MX" />
  <meta property="og:url" content="{SITE}/" />
  <meta property="og:title" content={title} />
  <meta property="og:description" content={description} />
  <meta property="og:image" content="{SITE}/og.png" />
  <meta name="twitter:card" content="summary_large_image" />
  {@html `<script type="application/ld+json">${jsonld}<` + `/script>`}
</svelte:head>

<div class="landing grain min-h-screen overflow-x-clip bg-paper font-body text-ink antialiased selection:bg-signal selection:text-white [-webkit-tap-highlight-color:transparent]" data-theme={theme.mode}>
  <Nav lang />
  <main>
  <Hero />
  <LazySection load={() => import('./Chaos.svelte')} height="90vh" />
  <LazySection load={() => import('./Features.svelte')} id="funciones" height="90vh" />
  <LazySection load={() => import('./Specialties.svelte')} id="especialidades" height="80vh" />
  <LazySection load={() => import('./Retention.svelte')} id="retencion" height="60vh" />
  <LazySection load={() => import('./Modules.svelte')} id="incluye" height="70vh" />
  <LazySection load={() => import('./Security.svelte')} id="seguridad" height="70vh" />
  <LazySection load={() => import('./Pricing.svelte')} id="precios" height="120vh" />
  <LazySection load={() => import('./Faq.svelte')} id="preguntas" height="60vh" />
  <LazySection load={() => import('./Cta.svelte')} id="contacto" height="55vh" />
  </main>
  <LazySection load={() => import('./Footer.svelte')} height="40vh" />
</div>
