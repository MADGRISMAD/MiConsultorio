<script lang="ts" generics="T">
  import { onMount, type Snippet } from 'svelte';
  import { reducedMotion } from './motion';

  interface Props {
    items: T[];
    label: (item: T) => string;
    panel: Snippet<[T, number]>;
    name: string;
    id: string;
    interval?: number;
  }
  /**
   * Pestañas que avanzan solas. La barra de la pestaña activa es el reloj: al terminar su
   * animación pasa a la siguiente. Se detiene con el puntero o el foco encima, fuera de pantalla
   * y con "reducir movimiento". En celular también se desliza con el dedo.
   */
  let { items, label, panel, name, id, interval = 6000 }: Props = $props();

  let active = $state(0);
  let hovering = $state(false);
  let focused = $state(false);
  let visible = $state(false);
  let auto = $state(false);
  let root: HTMLElement;
  const tabs: HTMLButtonElement[] = [];
  const running = $derived(auto && visible && !hovering && !focused);

  onMount(() => {
    auto = !reducedMotion();
    const io = new IntersectionObserver(([e]) => (visible = e.isIntersecting), { threshold: 0.35 });
    io.observe(root);
    return () => io.disconnect();
  });

  let list: HTMLElement;
  function go(i: number) {
    active = (i + items.length) % items.length;
    // En celular las pestañas se desplazan de lado: lleva la activa a la vista sin mover la página.
    const t = tabs[active];
    if (t && list) list.scrollTo({ left: t.offsetLeft - list.clientWidth / 2 + t.offsetWidth / 2, behavior: 'smooth' });
  }

  function onKey(e: KeyboardEvent) {
    const d = e.key === 'ArrowRight' ? 1 : e.key === 'ArrowLeft' ? -1 : 0;
    if (!d) return;
    e.preventDefault();
    go(active + d);
    tabs[active]?.focus();
  }

  let startX = 0;
  const onDown = (e: PointerEvent) => (startX = e.clientX);
  function onUp(e: PointerEvent) {
    const dx = e.clientX - startX;
    if (Math.abs(dx) > 50) go(active + (dx < 0 ? 1 : -1));
  }
</script>

<div
  bind:this={root}
  role="region"
  aria-roledescription="carrusel"
  aria-label={name}
  onpointerenter={(e) => e.pointerType === 'mouse' && (hovering = true)}
  onpointerleave={() => (hovering = false)}
  onfocusin={() => (focused = true)}
  onfocusout={() => (focused = false)}
>
  <div bind:this={list} role="tablist" aria-label={name} tabindex="-1" class="-mx-5 flex gap-2 overflow-x-auto px-5 pb-1 [scrollbar-width:none] sm:mx-0 sm:flex-wrap sm:px-0" onkeydown={onKey}>
    {#each items as item, i}
      <button
        bind:this={tabs[i]}
        role="tab"
        id="{id}-tab-{i}"
        aria-controls="{id}-panel-{i}"
        aria-selected={active === i}
        tabindex={active === i ? 0 : -1}
        class="relative flex-none overflow-hidden rounded-full px-4 py-2 text-[15px] font-medium transition-colors {active === i ? 'bg-ink text-paper' : 'bg-panel text-ink ring-1 ring-ink/15 hover:ring-ink/40'}"
        onclick={() => go(i)}
      >
        {label(item)}
        {#if active === i && auto}
          <span
            aria-hidden="true"
            class="carousel-clock absolute inset-x-0 bottom-0 h-[3px] origin-left bg-signal"
            style="animation-duration: {interval}ms; animation-play-state: {running ? 'running' : 'paused'}"
            onanimationend={() => go(active + 1)}
          ></span>
        {/if}
      </button>
    {/each}
  </div>
  <!-- Todas las tarjetas en la misma celda: el bloque mide lo que la más alta y la página no salta al cambiar. -->
  <div class="mt-5 grid touch-pan-y" role="presentation" onpointerdown={onDown} onpointerup={onUp}>
    {#each items as item, i}
      <div
        id="{id}-panel-{i}"
        role="tabpanel"
        aria-labelledby="{id}-tab-{i}"
        inert={active !== i}
        aria-hidden={active !== i}
        class="carousel-panel grid [grid-area:1/1] {active === i ? 'is-active' : 'invisible'}"
      >
        {@render panel(item, i)}
      </div>
    {/each}
  </div>
</div>

<style>
  .carousel-clock {
    animation-name: carousel-clock;
    animation-timing-function: linear;
  }
  @keyframes carousel-clock {
    from {
      transform: scaleX(0);
    }
    to {
      transform: scaleX(1);
    }
  }
  .carousel-panel.is-active {
    animation: carousel-in 0.5s cubic-bezier(0.22, 1, 0.36, 1);
  }
  @keyframes carousel-in {
    from {
      opacity: 0;
      transform: translateX(24px);
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .carousel-panel.is-active {
      animation: none;
    }
  }
</style>
