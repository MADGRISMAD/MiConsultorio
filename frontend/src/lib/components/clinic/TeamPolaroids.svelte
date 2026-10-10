<script lang="ts">
  import Icon from '$lib/components/ui/Icon.svelte';

  interface Person {
    name: string;
    title: string;
    photo_url: string;
  }
  let { people }: { people: Person[] } = $props();

  const n = $derived(people.length);
  const TILT = [-3, 2.5, -1.5, 3, -2.5, 1.5];
  const tilt = (i: number) => TILT[i % TILT.length];
  const initials = (s: string) => s.split(/\s+/).filter(Boolean).slice(0, 2).map((w) => w[0]?.toUpperCase()).join('');

  let index = $state(0); // the photo on top
  let leaving = $state(-1); // the one being passed away
  let jump = $state(-1); // moves to the back of the pile without animating
  let drag = $state(0);
  let dragging = $state(false);
  let startX = 0;

  /** where card i sits: 0 is the top of the pile, 1 and 2 peek out from behind, the rest wait hidden */
  const slot = (i: number) => (i - index + n) % n;

  function next() {
    if (n < 2 || leaving >= 0) return;
    leaving = index;
    index = (index + 1) % n;
    setTimeout(() => {
      jump = leaving;
      leaving = -1;
      requestAnimationFrame(() => requestAnimationFrame(() => (jump = -1)));
    }, 520);
  }
  function prev() {
    if (n < 2 || leaving >= 0) return;
    index = (index - 1 + n) % n;
  }

  function style(i: number): string {
    const s = slot(i);
    const t = tilt(i);
    if (i === leaving) return `transform: translate(-135%, 6%) rotate(${t - 16}deg); opacity: 0; z-index: ${n + 2};`;
    const depth = Math.min(s, 2);
    const x = s === 0 ? drag : depth * 26;
    const y = depth * 10;
    const rot = s === 0 ? t + drag / 28 : t + (s % 2 ? 4 : -3);
    const scale = 1 - depth * 0.045;
    const op = s > 2 ? 0 : 1;
    return `transform: translate(${x}px, ${y}px) rotate(${rot}deg) scale(${scale}); opacity: ${op}; z-index: ${n - s}; ${jump === i || (s === 0 && dragging) ? 'transition: none;' : ''}`;
  }

  function down(e: PointerEvent) {
    if (n < 2) return;
    dragging = true;
    startX = e.clientX;
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
  }
  function move(e: PointerEvent) {
    if (dragging) drag = e.clientX - startX;
  }
  function up() {
    if (!dragging) return;
    dragging = false;
    const d = drag;
    drag = 0;
    if (d < -50) next();
    else if (d > 50) prev();
  }
</script>

<div class="grid items-center gap-10 md:grid-cols-[1fr_auto]" role="region" aria-roledescription="carrusel" aria-label="Nuestro equipo">
  <div class="order-2 md:order-1">
    <p class="font-display text-[clamp(1.6rem,3vw,2.2rem)] leading-tight text-ink-soft">
      {#if n > 1}Pasa las fotos para <em class="text-ink">conocer</em> a quienes te atenderán.{:else}Quien te <em class="text-ink">atenderá</em>.{/if}
    </p>
    {#if n > 1}
      <div class="mt-6 flex items-center gap-3">
        <button type="button" class="grid h-12 w-12 place-items-center rounded-full bg-panel text-ink ring-1 ring-ink/15 transition hover:bg-ink hover:text-paper" aria-label="Anterior" onclick={prev}><Icon name="arrow-left" size={20} /></button>
        <button type="button" class="grid h-12 w-12 place-items-center rounded-full bg-ink text-paper transition hover:bg-signal" aria-label="Siguiente" onclick={next}><Icon name="arrow-right" size={20} /></button>
        <span class="ml-2 font-mono text-xs tracking-[0.14em] text-ink-faint" aria-live="polite">{index + 1} / {n}</span>
      </div>
    {/if}
  </div>

  <div class="order-1 mx-auto md:order-2">
    <ul
      class="pile relative mx-auto h-[26rem] w-[17.5rem] touch-pan-y select-none sm:h-[28rem] sm:w-[19rem]"
      class:cursor-grab={n > 1}
      onpointerdown={down}
      onpointermove={move}
      onpointerup={up}
      onpointercancel={up}
    >
      {#each people as p, i (p.name + i)}
        <li class="polaroid absolute inset-0 flex flex-col rounded-[6px] bg-white p-3 pb-0 shadow-[0_22px_40px_-14px_rgba(11,37,64,0.45),0_2px_6px_rgba(11,37,64,0.18)]" style={style(i)} aria-hidden={slot(i) !== 0} aria-label={p.name}>
          <span class="pointer-events-none absolute -top-3 left-1/2 h-6 w-20 -translate-x-1/2 rotate-[-3deg] bg-signal/25 backdrop-blur-[1px]" aria-hidden="true"></span>
          <div class="min-h-0 flex-1 overflow-hidden rounded-[2px] bg-signal-soft">
            {#if p.photo_url}<img src={p.photo_url} alt={slot(i) === 0 ? `Foto de ${p.name}` : ''} class="h-full w-full object-cover" draggable="false" loading="lazy" />{:else}<span class="grid h-full place-items-center font-display text-7xl text-signal">{initials(p.name)}</span>{/if}
          </div>
          <div class="flex h-[5.2rem] flex-col justify-center px-1 text-center">
            <p class="font-display text-[1.7rem] italic leading-none text-ink">{p.name}</p>
            {#if p.title}<p class="mt-1.5 truncate font-mono text-[10.5px] uppercase tracking-[0.12em] text-ink-soft">{p.title}</p>{/if}
          </div>
        </li>
      {/each}
    </ul>
  </div>
</div>

<style>
  .polaroid {
    transition: transform 520ms cubic-bezier(0.22, 1, 0.36, 1), opacity 420ms ease;
    transform-origin: 50% 90%;
    backface-visibility: hidden;
  }
  @media (prefers-reduced-motion: reduce) {
    .polaroid {
      transition: opacity 200ms ease;
    }
  }
</style>
