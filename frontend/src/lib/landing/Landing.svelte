<script lang="ts">
  import { onMount } from 'svelte';
  import Lenis from 'lenis';
  import Hero from './Hero.svelte';
  import LazySection from './LazySection.svelte';
  import Nav from './Nav.svelte';
  import { theme } from '$lib/theme.svelte';
  import { reducedMotion } from './motion';

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

<div class="landing grain min-h-screen overflow-x-clip bg-paper font-body text-ink antialiased selection:bg-signal selection:text-white [-webkit-tap-highlight-color:transparent]" data-theme={theme.mode}>
  <Nav />
  <main>
  <Hero />
  <LazySection load={() => import('./Chaos.svelte')} height="90vh" />
  <LazySection load={() => import('./Features.svelte')} id="funciones" height="120vh" />
  <LazySection load={() => import('./Retention.svelte')} id="retencion" />
  <LazySection load={() => import('./Modules.svelte')} id="incluye" height="100vh" />
  <LazySection load={() => import('./Marquee.svelte')} height="20vh" />
  <LazySection load={() => import('./Specialties.svelte')} id="especialidades" height="100vh" />
  <LazySection load={() => import('./Security.svelte')} id="seguridad" />
  <LazySection load={() => import('./Steps.svelte')} height="80vh" />
  <div class="relative z-10 -mt-9 rounded-t-[36px] bg-paper">
    <LazySection load={() => import('./Pricing.svelte')} id="precios" height="100vh" />
    <LazySection load={() => import('./Faq.svelte')} id="preguntas" />
  </div>
  <LazySection load={() => import('./Cta.svelte')} id="contacto" height="70vh" />
  </main>
  <LazySection load={() => import('./Footer.svelte')} height="40vh" />
</div>
