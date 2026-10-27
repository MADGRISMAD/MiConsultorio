<script lang="ts">
  import type { BlogBlock } from '$lib/api/blog';
  import Rich from './Rich.svelte';

  // Cuerpo y preguntas frecuentes de un artículo del blog, con la tipografía de la portada.
  let { body, faqs }: { body: BlogBlock[]; faqs: { q: string; a: string }[] } = $props();

  let openFaq = $state<number | null>(0);
</script>

<div class="article text-[17px] leading-[1.75] text-ink-soft sm:text-[18px]">
  {#each body as b, i (i)}
    {#if b.type === 'h2'}
      <h2 class="mt-14 font-display text-[clamp(1.9rem,4vw,2.6rem)] font-normal leading-[1.05] tracking-[-0.02em] text-ink first:mt-0">{b.text}</h2>
    {:else if b.type === 'h3'}
      <h3 class="mt-9 text-[20px] font-semibold leading-snug tracking-[-0.01em] text-ink">{b.text}</h3>
    {:else if b.type === 'p'}
      <p class="mt-5 [text-wrap:pretty]"><Rich text={b.text ?? ''} /></p>
    {:else if b.type === 'ul' || b.type === 'ol'}
      <svelte:element this={b.type} class="mt-5 space-y-3">
        {#each b.items ?? [] as item, n}
          <li class="flex gap-3.5">
            {#if b.type === 'ol'}
              <span class="mt-[3px] grid h-6 w-6 flex-none place-items-center rounded-full bg-signal-soft font-mono text-[12px] tabular-nums text-signal">{n + 1}</span>
            {:else}
              <span class="mt-[11px] h-1.5 w-1.5 flex-none rounded-full bg-signal"></span>
            {/if}
            <span class="[text-wrap:pretty]"><Rich text={item} /></span>
          </li>
        {/each}
      </svelte:element>
    {:else if b.type === 'tip'}
      <aside class="mt-8 rounded-[20px] border border-mint/25 bg-mint-soft/60 px-5 py-4 sm:px-6 sm:py-5">
        <p class="font-mono text-[11px] uppercase tracking-[0.16em] text-mint">Consejo</p>
        <p class="mt-2 text-ink [text-wrap:pretty]"><Rich text={b.text ?? ''} /></p>
      </aside>
    {/if}
  {/each}

  {#if faqs.length}
    <section aria-labelledby="faq" class="mt-16 border-t border-ink/10 pt-12">
      <h2 id="faq" class="font-display text-[clamp(1.9rem,4vw,2.6rem)] font-normal leading-[1.05] tracking-[-0.02em] text-ink">Preguntas frecuentes</h2>
      <ul class="mt-6 border-t border-ink/10">
        {#each faqs as f, i}
          {@const isOpen = openFaq === i}
          <li class="border-b border-ink/10">
            <button type="button" aria-expanded={isOpen} onclick={() => (openFaq = isOpen ? null : i)} class="group flex w-full items-center justify-between gap-6 py-5 text-left">
              <span class="text-[17px] font-medium text-ink transition-colors group-hover:text-signal">{f.q}</span>
              <span class="relative grid h-8 w-8 flex-none place-items-center rounded-full transition-colors {isOpen ? 'bg-ink text-paper' : 'text-ink ring-1 ring-ink/20'}">
                <span class="absolute h-[1.5px] w-3 bg-current"></span>
                <span class="absolute h-3 w-[1.5px] bg-current transition-transform duration-300 {isOpen ? 'rotate-90' : ''}"></span>
              </span>
            </button>
            <!-- la respuesta siempre está en el HTML (buscadores y asistentes de IA la leen); solo se oculta mientras la pregunta está cerrada -->
            <p class="pb-5 pr-10 [text-wrap:pretty]" hidden={!isOpen}>{f.a}</p>
          </li>
        {/each}
      </ul>
    </section>
  {/if}
</div>
