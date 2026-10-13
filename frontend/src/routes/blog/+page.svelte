<script lang="ts">
  // Blog alojado en Soro (app.trysoro.com): su script pinta los artículos dentro de #soro-blog.
  // La llave es el identificador del sitio en Soro; para cambiar de cuenta solo se cambia SORO_KEY.
  import { onMount } from 'svelte';
  import { theme } from '$lib/theme.svelte';
  import Footer from '$lib/landing/Footer.svelte';
  import Nav from '$lib/landing/Nav.svelte';

  const SORO_KEY = 'f89f47a2-4b3e-4fd4-8dbc-e856c6ebb5a6';
  const SORO_SRC = `https://app.trysoro.com/api/embed/${SORO_KEY}`;

  const title = 'Blog · Caresia';
  const description = 'Guías y consejos para administrar tu clínica dental, consultorio médico, nutrición, veterinaria y más: agenda, expedientes, pacientes y crecimiento.';

  $effect(() => theme.init());

  let box: HTMLDivElement;
  onMount(() => {
    // La app es de una sola página: el script se inyecta al entrar a /blog (también al llegar navegando desde
    // otra página) y se retira al salir, para que la siguiente visita lo vuelva a ejecutar.
    const s = document.createElement('script');
    s.src = SORO_SRC;
    s.defer = true;
    document.body.appendChild(s);
    return () => {
      s.remove();
      box?.replaceChildren();
    };
  });
</script>

<svelte:head>
  <title>{title}</title>
  <meta name="description" content={description} />
  <meta property="og:type" content="website" />
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

    <div class="mt-14 border-t border-ink/10 pt-12">
      <div id="soro-blog" bind:this={box}></div>
      <noscript><p class="text-ink-soft">Activa JavaScript para leer el blog.</p></noscript>
    </div>
  </main>

  <Footer home="/" />
</div>
