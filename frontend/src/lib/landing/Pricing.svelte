<script lang="ts">
  import ButtonLabel from './ButtonLabel.svelte';
  import Icon from './Icon.svelte';
  import Split from './Split.svelte';
  import { demoHref, plans } from './data';
  import { btn, reveal } from './motion';

  // The spotlight follows the pointer via --x / --y
  function spotlight(e: PointerEvent) {
    const el = e.currentTarget as HTMLElement;
    const r = el.getBoundingClientRect();
    el.style.setProperty('--x', `${e.clientX - r.left}px`);
    el.style.setProperty('--y', `${e.clientY - r.top}px`);
  }
</script>

<section id="precios" class="mx-auto max-w-6xl px-5 py-28 sm:px-8 lg:py-40">
  <div class="grid gap-6 border-b border-ink/15 pb-10 md:grid-cols-[1.7fr_1fr] md:items-end">
    <Split text={'Planes claros,\n*sin letras chiquitas.*'} class="font-display text-[clamp(2.75rem,6.5vw,5.25rem)] leading-[0.95] tracking-[-0.03em]" accent="italic text-signal" />
    <p class="max-w-sm text-lg leading-relaxed text-ink-soft md:justify-self-end">Pago mensual, sin plazo forzoso. Cancela cuando quieras.</p>
  </div>
  <ul class="mt-14 grid gap-4 lg:mt-20 lg:grid-cols-3 lg:items-stretch">
    {#each plans as p, i}
      {@const dark = p.featured}
      <li class="flex">
        <div
          role="presentation"
          use:reveal={{ y: 60, delay: i * 0.1, margin: '0px 0px -10% 0px' }}
          onpointermove={spotlight}
          class="group relative flex w-full flex-col overflow-hidden rounded-[28px] p-8 sm:p-9 {dark ? 'l-deep bg-ink text-paper lg:-my-6 lg:py-14' : 'bg-panel ring-1 ring-ink/10'}"
        >
          <span
            aria-hidden="true"
            class="pointer-events-none absolute inset-0 opacity-0 transition-opacity duration-300 group-hover:opacity-100"
            style="background: radial-gradient(320px circle at var(--x) var(--y), {dark ? 'rgba(22,115,209,0.22)' : 'rgba(22,115,209,0.10)'}, transparent 70%)"
          ></span>
          <div class="relative flex items-center justify-between">
            <h3 class="font-display text-4xl">{p.name}</h3>
            {#if p.featured}<span class="rounded-full bg-signal px-3 py-1 font-mono text-[10px] uppercase tracking-[0.14em] text-white">Recomendado</span>{/if}
          </div>
          <p class="relative mt-1 font-mono text-[11px] uppercase tracking-[0.12em] {dark ? 'text-paper/50' : 'text-ink-faint'}">{p.tagline}</p>
          <p class="relative mt-6 flex items-baseline gap-2">
            <span class="font-display text-6xl leading-none tracking-[-0.02em] tabular-nums">{p.price}</span>
            {#if p.period}<span class="font-mono text-[11px] uppercase tracking-[0.1em] {dark ? 'text-paper/50' : 'text-ink-faint'}">{p.period}</span>{/if}
          </p>
          <p class="relative mt-5 rounded-2xl px-4 py-3 text-[14px] {dark ? 'bg-paper/8 text-paper/85' : 'bg-ink/5 text-ink'}"><span class="font-semibold">{p.magic} usos de magia</span> al mes: plan nutricional, resumen de consulta con IA, inventario y precios.</p>
          <p class="relative mt-5 text-[15px] leading-relaxed {dark ? 'text-paper/60' : 'text-ink-soft'}">{p.blurb}</p>
          {#if p.includes}<p class="relative mt-6 text-[13px] font-semibold {dark ? 'text-paper' : 'text-ink'}">{p.includes}</p>{/if}
          <ul class="relative mt-4 flex-1 space-y-3 text-[15px]">
            {#each p.features as f}
              <li class="flex gap-3">
                <Icon name="check" class="mt-0.5 h-4 w-4 flex-none {dark ? 'text-signal' : 'text-mint'}" strokeWidth={2.4} />
                <span class={dark ? 'text-paper/85' : 'text-ink'}>{f}</span>
              </li>
            {/each}
          </ul>
          <a href={demoHref} class="{btn} relative mt-10 h-12 px-6 text-[15px] {dark ? 'bg-signal text-white hover:bg-paper hover:text-ink focus-visible:ring-offset-ink' : 'bg-ink text-paper hover:bg-signal'}">
            <ButtonLabel>{p.cta}</ButtonLabel>
          </a>
        </div>
      </li>
    {/each}
  </ul>
  <p class="mt-10 text-center font-mono text-[11px] uppercase tracking-[0.14em] text-ink-faint">Precios en pesos mexicanos · IVA no incluido</p>
</section>
