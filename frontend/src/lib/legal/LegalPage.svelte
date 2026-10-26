<script lang="ts">
  import { page } from '$app/state';
  import type { Snippet } from 'svelte';
  import { theme } from '$lib/theme.svelte';
  import Footer from '$lib/landing/Footer.svelte';
  import Nav from '$lib/landing/Nav.svelte';
  import Fill from './Fill.svelte';
  import { isPending, OPERATOR } from './operator';

  // Páginas legales con el mismo diseño que la portada: su menú, su pie, sus tipografías y colores.
  let { title, intro, toc, children }: { title: string; intro: string; toc: { id: string; label: string }[]; children: Snippet } = $props();

  $effect(() => theme.init());

  /** Mientras falte un dato del responsable, se avisa arriba y el dato se resalta en el texto. */
  const pendiente = Object.values(OPERATOR).some(isPending);
</script>

<svelte:head>
  <title>{title} · Caresia</title>
  <meta name="description" content={intro} />
  <link rel="canonical" href="https://caresia.mx{page.url.pathname}" />
</svelte:head>

<div class="landing grain min-h-screen overflow-x-clip bg-paper font-body text-ink antialiased selection:bg-signal selection:text-white" data-theme={theme.mode}>
  <Nav home="/" />

  <main class="mx-auto max-w-6xl px-5 pb-28 pt-32 sm:px-8 sm:pt-40">
    <p class="flex items-center gap-2 font-mono text-[11px] uppercase tracking-[0.16em] text-ink-soft">
      <span class="h-1.5 w-1.5 rounded-full bg-signal"></span>Legal
    </p>
    <h1 class="mt-6 font-display text-[clamp(3rem,10vw,6.5rem)] font-normal leading-[0.92] tracking-[-0.035em]">{title}</h1>
    <p class="mt-6 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-soft">Última actualización: <Fill v={OPERATOR.actualizado} /></p>
    <p class="mt-6 max-w-2xl text-lg leading-relaxed text-ink-soft [text-wrap:pretty]">{intro}</p>

    {#if pendiente}
      <p class="mt-6 max-w-2xl rounded-2xl bg-[#f59e0b]/12 px-5 py-4 text-[15px] text-[#b45309] ring-1 ring-[#f59e0b]/30" role="note">
        Faltan datos del responsable por completar; aparecen resaltados en el texto.
      </p>
    {/if}

    <div class="mt-14 grid gap-12 border-t border-ink/10 pt-12 lg:grid-cols-[17rem_minmax(0,1fr)] lg:gap-20">
      <nav aria-label="Contenido" class="lg:sticky lg:top-28 lg:self-start">
        <p class="font-mono text-[11px] uppercase tracking-[0.16em] text-ink-soft">Contenido</p>
        <ol class="mt-4 space-y-2.5 text-[15px] leading-snug">
          {#each toc as t, i (t.id)}
            <li>
              <a href={`#${t.id}`} class="group flex gap-3 text-ink-soft transition-colors hover:text-ink">
                <span class="pt-[3px] font-mono text-[11px] tabular-nums text-ink-faint group-hover:text-signal">{String(i + 1).padStart(2, '0')}</span>
                <span>{t.label}</span>
              </a>
            </li>
          {/each}
        </ol>
      </nav>

      <div class="legal max-w-2xl text-ink-soft">{@render children()}</div>
    </div>
  </main>

  <Footer home="/" />
</div>

<style>
  .legal :global(h2) {
    margin-bottom: 0.9rem;
    scroll-margin-top: 7rem;
    font-size: clamp(1.7rem, 3.2vw, 2.25rem);
    line-height: 1.05;
    letter-spacing: -0.02em;
    color: rgb(var(--l-ink));
  }
  .legal :global(section + section) {
    margin-top: 3.25rem;
  }
  .legal :global(p),
  .legal :global(li) {
    font-size: 16px;
    line-height: 1.75;
  }
  .legal :global(strong) {
    font-weight: 600;
    color: rgb(var(--l-ink));
  }
  .legal :global(a) {
    color: rgb(var(--l-signal));
    text-decoration: underline;
    text-underline-offset: 3px;
  }
  .legal :global(code) {
    font-family: var(--font-mono, ui-monospace, monospace);
    font-size: 0.9em;
  }
</style>
