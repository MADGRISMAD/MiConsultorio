<script lang="ts">
  import type { Snippet } from 'svelte';
  import { theme } from '$lib/theme.svelte';
  import Brand from '$lib/components/ui/Brand.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import Fill from './Fill.svelte';
  import { OPERATOR } from './operator';

  let { title, intro, toc, children }: { title: string; intro: string; toc: { id: string; label: string }[]; children: Snippet } = $props();

  $effect(() => theme.init());
</script>

<svelte:head><title>{title} · Caresia</title></svelte:head>

<div class="app fixed inset-0 overflow-y-auto overflow-x-hidden px-4 py-8 sm:py-12" data-theme={theme.mode}>
  <article class="card page-in mx-auto max-w-3xl px-5 py-8 sm:px-10">
    <a href="/" class="inline-flex" aria-label="Caresia, ir al inicio"><Brand size={32} class="text-[1.05rem]" /></a>

    <!-- BORRADOR: retirar este aviso solo cuando un abogado haya revisado el texto y se hayan completado los datos del operador. -->
    <p class="mt-6 flex items-start gap-2.5 rounded-xl bg-app-warning/12 px-4 py-3 text-sm font-medium text-app-warning" role="note">
      <Icon name="alert" size={18} class="mt-0.5 flex-none" />
      <span>Borrador pendiente de revisión legal. Este texto aún no es la versión definitiva y los datos marcados en amarillo están por completarse.</span>
    </p>

    <h1 class="display mt-7 text-4xl leading-none sm:text-5xl">{title}</h1>
    <p class="mt-3 text-sm text-app-muted">Última actualización: <Fill v={OPERATOR.actualizado} /></p>
    <p class="mt-4 text-[15px] leading-relaxed text-app-muted">{intro}</p>

    <nav class="mt-6 rounded-xl bg-app-ink/5 px-5 py-4" aria-label="Contenido">
      <p class="section-title">Contenido</p>
      <ol class="mt-2 grid gap-x-6 gap-y-1 text-sm sm:grid-cols-2">
        {#each toc as t, i (t.id)}
          <li><a href={`#${t.id}`} class="underline decoration-app-ink/20 underline-offset-2 hover:decoration-app-ink">{i + 1}. {t.label}</a></li>
        {/each}
      </ol>
    </nav>

    <div class="legal mt-8 space-y-8 text-[15px] leading-relaxed">{@render children()}</div>

    <a href="/" class="btn-secondary mt-10">← Volver al inicio</a>
  </article>
</div>

<style>
  .legal :global(h2) {
    margin-bottom: 0.6rem;
    scroll-margin-top: 1.5rem;
  }
  .legal :global(p + p),
  .legal :global(p + ul),
  .legal :global(ul + p) {
    margin-top: 0.75rem;
  }
  .legal :global(ul) {
    list-style: disc;
    padding-left: 1.25rem;
  }
  .legal :global(li + li) {
    margin-top: 0.35rem;
  }
  .legal :global(a) {
    text-decoration: underline;
  }
</style>
