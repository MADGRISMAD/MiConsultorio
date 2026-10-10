<script lang="ts">
  import { onMount } from 'svelte';
  import Icon from '$lib/components/ui/Icon.svelte';

  let { photos, label = 'Fotos del consultorio' }: { photos: string[]; label?: string } = $props();

  let track = $state<HTMLElement>();
  let index = $state(0);
  let again = $state(0);

  const go = (i: number) => {
    if (!track) return;
    const n = photos.length;
    const to = (i + n) % n;
    track.scrollTo({ left: to * track.clientWidth, behavior: 'smooth' });
    index = to;
  };
  function onScroll() {
    if (!track) return;
    index = Math.round(track.scrollLeft / Math.max(1, track.clientWidth));
  }

  // Changes by itself every 5 s. Every change (automatic, arrows, dots or swipe) restarts the countdown,
  // and a manual change never stops the automatic one.
  $effect(() => {
    void index;
    void again;
    if (photos.length < 2 || window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    const t = setTimeout(() => {
      if (document.hidden) again++; // hidden tab: look again in 5 s
      else go(index + 1);
    }, 5000);
    return () => clearTimeout(t);
  });
</script>

<div class="relative" role="region" aria-roledescription="carrusel" aria-label={label}>
  <div bind:this={track} class="flex snap-x snap-mandatory overflow-x-auto scroll-smooth rounded-[28px] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden" onscroll={onScroll}>
    {#each photos as src, i (src)}
      <div class="aspect-[16/9] w-full flex-none snap-center sm:aspect-[2/1]" role="group" aria-roledescription="foto" aria-label="Foto {i + 1} de {photos.length}">
        <img {src} alt="{label}: foto {i + 1}" class="h-full w-full object-cover" loading={i === 0 ? 'eager' : 'lazy'} />
      </div>
    {/each}
  </div>
  {#if photos.length > 1}
    <button type="button" class="absolute left-3 top-1/2 grid h-10 w-10 -translate-y-1/2 place-items-center rounded-full bg-panel/90 text-ink shadow-lg ring-1 ring-ink/10 transition hover:bg-panel" aria-label="Foto anterior" onclick={() => go(index - 1)}><Icon name="arrow-left" size={18} /></button>
    <button type="button" class="absolute right-3 top-1/2 grid h-10 w-10 -translate-y-1/2 place-items-center rounded-full bg-panel/90 text-ink shadow-lg ring-1 ring-ink/10 transition hover:bg-panel" aria-label="Foto siguiente" onclick={() => go(index + 1)}><Icon name="arrow-right" size={18} /></button>
    <div class="absolute inset-x-0 bottom-3 flex justify-center gap-1.5">
      {#each photos as _, i}
        <button type="button" class="h-2 rounded-full transition-all {i === index ? 'w-6 bg-white' : 'w-2 bg-white/60'}" aria-label="Ir a la foto {i + 1}" aria-current={i === index} onclick={() => go(i)}></button>
      {/each}
    </div>
  {/if}
</div>
