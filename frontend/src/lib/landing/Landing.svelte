<script lang="ts">
  import { onMount } from 'svelte';
  import Lenis from 'lenis';
  import Chaos from './Chaos.svelte';
  import Cta from './Cta.svelte';
  import Faq from './Faq.svelte';
  import Features from './Features.svelte';
  import Footer from './Footer.svelte';
  import Hero from './Hero.svelte';
  import Marquee from './Marquee.svelte';
  import Nav from './Nav.svelte';
  import Pricing from './Pricing.svelte';
  import Specialties from './Specialties.svelte';
  import Steps from './Steps.svelte';
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

<div class="grain overflow-x-clip bg-paper font-body text-ink antialiased selection:bg-signal selection:text-white [-webkit-tap-highlight-color:transparent]">
  <Nav />
  <Hero />
  <Chaos />
  <Features />
  <Marquee />
  <Specialties />
  <Steps />
  <div class="relative z-10 -mt-9 rounded-t-[36px] bg-paper">
    <Pricing />
    <Faq />
  </div>
  <Cta />
  <Footer />
</div>
